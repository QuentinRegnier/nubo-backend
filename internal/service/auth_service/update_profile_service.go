package auth_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/cuckoo"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
)

// UpdateProfile modifie l'identité, vérifie l'unicité, gère l'avatar et envoie au Write-Behind.
func UpdateProfile(ctx context.Context, userID int64, input auth_models.UpdateProfileInput) (auth_models.UpdateProfileOutput, error) {
	// 1. Récupération de l'utilisateur existant complet (L2 -> L3) pour préserver les données critiques
	user, err := mongo.MongoLoadUser(userID, "", "", "")
	if err != nil || user.ID == 0 {
		user, err = postgres.FuncLoadUser(userID, "", "", "")
		if err != nil || user.ID == 0 {
			return auth_models.UpdateProfileOutput{}, nubo_error.NewNotFound("USER_NOT_FOUND", "Utilisateur introuvable.", err)
		}
	}

	// Variables pour le Cuckoo Filter (pour retirer les anciens et ajouter les nouveaux)
	oldUsername, oldEmail, oldPhone := "", "", ""
	newUsername, newEmail, newPhone := "", "", ""

	// 2. Vérification d'unicité uniquement si la valeur a changé
	if input.Username != user.Username {
		if service.IsUnique(ctx, redis.EntityUser, "username", input.Username) == 0 {
			return auth_models.UpdateProfileOutput{}, nubo_error.NewConflict("USERNAME_TAKEN", "Ce nom d'utilisateur est déjà pris.", nil)
		}
		oldUsername = user.Username
		newUsername = input.Username
		user.Username = input.Username
	}

	if input.Email != user.Email {
		if service.IsUnique(ctx, redis.EntityUser, "email", input.Email) == 0 {
			return auth_models.UpdateProfileOutput{}, nubo_error.NewConflict("EMAIL_TAKEN", "Cet email est déjà utilisé.", nil)
		}
		oldEmail = user.Email
		newEmail = input.Email
		user.Email = input.Email
		user.EmailVerified = false // Nécessite une nouvelle vérification
	}

	// === NOUVEAU BLOC : GESTION DU TÉLÉPHONE ===
	if input.Phone != user.Phone {
		if input.Phone != "" && service.IsUnique(ctx, redis.EntityUser, "phone", input.Phone) == 0 {
			return auth_models.UpdateProfileOutput{}, nubo_error.NewConflict("PHONE_TAKEN", "Ce numéro de téléphone est déjà utilisé.", nil)
		}
		oldPhone = user.Phone
		newPhone = input.Phone
		user.Phone = input.Phone
		user.PhoneVerified = false // Nécessite une nouvelle vérification SMS
	}

	// 3. Remplacement intégral des autres champs
	user.FirstName = pkg.CleanStr(input.FirstName)
	user.LastName = pkg.CleanStr(input.LastName)
	user.Bio = pkg.CleanStr(input.Bio)
	user.Location = pkg.CleanStr(input.Location)
	user.School = pkg.CleanStr(input.School)
	user.Work = pkg.CleanStr(input.Work)

	// === GESTION DES MÉDIAS EN BATCH ===
	if input.ProfilePictureID != user.ProfilePictureID {
		// A. Tuer l'ancien fantôme (S'il en avait un)
		if user.ProfilePictureID > 0 {
			_ = media_service.DeactivateMediaBatch(ctx, []int64{user.ProfilePictureID}, userID)
		}

		// B. Activer la nouvelle image
		if input.ProfilePictureID > 0 {
			if errAct := media_service.ActivateMediaBatch(ctx, []int64{input.ProfilePictureID}, userID); errAct != nil {
				return auth_models.UpdateProfileOutput{}, nubo_error.NewBadRequest("AVATAR_ACTIVATION_FAILED", "Impossible de valider la nouvelle photo de profil.", errAct)
			}
		}
		user.ProfilePictureID = input.ProfilePictureID
	}

	user.UpdatedAt = service.NowMillis()

	// 5. Mise à jour immédiate du Speed Cache L1 (Auto-complétion et Profil Lite)
	// CORRECTION : Récupération asynchrone/rapide des settings pour construire l'objet Lite
	settings, _ := object_cache_service.GetUserSettingsCascade(ctx, userID)
	_ = cache_service.AddUserToSpeedCache(ctx, user, settings)

	// 6. Mise à jour du Cuckoo Filter via Flux Redis (Asynchrone et Distribué)
	if newUsername != "" {
		cuckoo.BroadcastCuckooUpdate(cuckoo.ActionAdd, "username", newUsername)
		cuckoo.BroadcastCuckooUpdate(cuckoo.ActionDel, "username", oldUsername)
	}
	if newEmail != "" {
		cuckoo.BroadcastCuckooUpdate(cuckoo.ActionAdd, "email", newEmail)
		cuckoo.BroadcastCuckooUpdate(cuckoo.ActionDel, "email", oldEmail)
	}
	if newPhone != "" {
		cuckoo.BroadcastCuckooUpdate(cuckoo.ActionAdd, "phone", newPhone)
		cuckoo.BroadcastCuckooUpdate(cuckoo.ActionDel, "phone", oldPhone)
	}

	// 7. Envoi notification
	_ = realtime_service.DistributeToUsers(ctx, "user.profile_updated", user, []int64{userID})

	// 8. Persistance Asynchrone (Write-Behind vers Mongo et Postgres avec l'objet complet)
	return auth_models.UpdateProfileOutput{
		ProfileUpdatedAt: user.UpdatedAt,
	}, redis.EnqueueDB(ctx, user.ID, 0, redis.EntityUser, redis.ActionUpdate, user, redis.TargetAll)
}
