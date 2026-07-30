package auth_service

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/cuckoo"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
)

// UpdateProfile modifie l'identité, vérifie l'unicité, gère l'avatar et envoie au Write-Behind.
func UpdateProfile(ctx context.Context, userID int64, input auth_models.UpdateProfileInput, fileHeader *multipart.FileHeader) error {
	// 1. Récupération de l'utilisateur existant complet (L2 -> L3) pour préserver les données critiques
	user, err := mongo.MongoLoadUser(userID, "", "", "")
	if err != nil || user.ID == 0 {
		user, err = postgres.FuncLoadUser(userID, "", "", "")
		if err != nil || user.ID == 0 {
			return errors.New("utilisateur introuvable")
		}
	}

	// Variables pour le Cuckoo Filter (pour retirer les anciens et ajouter les nouveaux)
	oldUsername, oldEmail, oldPhone := "", "", ""
	newUsername, newEmail, newPhone := "", "", ""

	// 2. Vérification d'unicité et application
	if input.Username != nil && *input.Username != user.Username {
		if service.IsUnique(mongo.Users, "username", *input.Username) == 0 {
			return errors.New("ce nom d'utilisateur est déjà pris")
		}
		oldUsername = user.Username
		newUsername = *input.Username
		user.Username = *input.Username
	}

	if input.Email != nil && *input.Email != user.Email {
		if service.IsUnique(mongo.Users, "email", *input.Email) == 0 {
			return errors.New("cet email est déjà utilisé")
		}
		oldEmail = user.Email
		newEmail = *input.Email
		user.Email = *input.Email
		user.EmailVerified = false // Nécessite une nouvelle vérification
	}

	if input.Phone != nil && *input.Phone != user.Phone {
		if service.IsUnique(mongo.Users, "phone", *input.Phone) == 0 {
			return errors.New("ce numéro de téléphone est déjà utilisé")
		}
		oldPhone = user.Phone
		newPhone = *input.Phone
		user.Phone = *input.Phone
		user.PhoneVerified = false // Nécessite une nouvelle vérification
	}

	// 3. Application des autres champs non-uniques
	if input.FirstName != nil {
		user.FirstName = *input.FirstName
	}
	if input.LastName != nil {
		user.LastName = *input.LastName
	}
	if input.Bio != nil {
		user.Bio = pkg.CleanStr(*input.Bio)
	}
	if input.Location != nil {
		user.Location = *input.Location
	}
	if input.School != nil {
		user.School = *input.School
	}
	if input.Work != nil {
		user.Work = *input.Work
	}

	// 4. Gestion de la photo de profil (MinIO S3)
	if fileHeader != nil {
		file, err := fileHeader.Open()
		if err != nil {
			return fmt.Errorf("impossible de lire le fichier: %w", err)
		}
		defer func(file multipart.File) {
			err := file.Close()
			if err != nil {
				fmt.Printf("Error en la fichier: %v", err)
			}
		}(file)

		oldMediaID := user.ProfilePictureID
		newMediaID := pkg.GenerateID()

		// Délégation de l'upload au domaine Média
		if errUpload := media_service.UploadMedia(file, userID, newMediaID); errUpload != nil {
			return fmt.Errorf("erreur upload avatar: %w", errUpload)
		}
		user.ProfilePictureID = newMediaID

		// Délégation propre de la suppression au domaine Média (Respect du DDD)
		if oldMediaID != 0 {
			_ = media_service.DeleteMedia(ctx, oldMediaID, userID)
		}
	}

	user.UpdatedAt = time.Now().UTC()

	// 5. Mise à jour immédiate du Speed Cache L1 (Auto-complétion et Profil Lite)
	_ = cache_service.AddUserToSpeedCache(ctx, user)

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

	// 7. Persistance Asynchrone (Write-Behind vers Mongo et Postgres avec l'objet complet)
	return redis.EnqueueDB(ctx, user.ID, 0, redis.EntityUser, redis.ActionUpdate, user, redis.TargetAll)
}
