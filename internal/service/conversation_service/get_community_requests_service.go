package conversation_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// GetCommunityRequests récupère les candidatures en attente d'approbation (Role = -3).
func GetCommunityRequests(ctx context.Context, callerID int64, input conversation_models.GetCommunityRequestsInput) (conversation_models.GetCommunityRequestsOutput, error) {
	// 1. SÉCURITÉ : Vérification des droits d'administration
	callerMem, err := security_service.LeftMember(ctx, input.ConversationID, callerID)
	if err != nil {
		return conversation_models.GetCommunityRequestsOutput{}, nubo_error.NewForbidden("ACCESS_DENIED", "Conversation introuvable ou accès refusé.", err)
	}

	if callerMem.Role < 1 { // Doit être au moins Admin (1) ou Owner (2)
		return conversation_models.GetCommunityRequestsOutput{}, nubo_error.NewForbidden("INSUFFICIENT_PERMISSIONS", "Vous devez être administrateur pour voir les demandes d'adhésion.", nil)
	}

	// 2. RÉCUPÉRATION DES MEMBRES (-3) VIA L2 -> L3
	var members []conversation_models.MemberPayload

	// Tentative L2 (MongoDB)
	mongoMembers, errMongo := mongo.MongoLoadMembersByRolePaginated(input.ConversationID, -3, input.Limit, input.Offset)
	if errMongo == nil && len(mongoMembers) > 0 {
		members = mongoMembers
	} else {
		// Fallback L3 (PostgreSQL)
		pgMembers, errPg := postgres.FuncLoadMembersByRolePaginated(ctx, input.ConversationID, -3, input.Limit, input.Offset)
		if errPg != nil {
			return conversation_models.GetCommunityRequestsOutput{}, nubo_error.NewInternal(errPg)
		}
		members = pgMembers

		// Auto-Guérison L2
		for _, m := range members {
			go func(member conversation_models.MemberPayload) {
				bgCtx := context.Background()
				// On l'envoie en tant qu'Update à Mongo pour qu'il le sauvegarde
				_ = redis.EnqueueDB(bgCtx, member.ID, member.ConversationID, redis.EntityMembers, redis.ActionUpdate, member, redis.TargetMongo)
			}(m)
		}
	}

	// 3. HYDRATATION MASSIVE VIA SPEED CACHE (L1) ET GÉNÉRATION DES MÉDIAS
	var requests []auth_models.UserLiteView

	for _, m := range members {
		// Fetch O(1) depuis le cache L1 (avec auto-fallback intégré dans GetUserLite si cache miss)
		userLite, errLite := cache_service.GetUserLite(ctx, m.UserID)
		if errLite != nil {
			continue // On ignore silencieusement les utilisateurs introuvables/supprimés
		}

		var avatarView media_models.MediaView
		if userLite.ProfilePictureID > 0 {
			// Signature HMAC via le Domaine Média
			if view, errMedia := media_service.GenerateMediaViewCascade(ctx, userLite.ProfilePictureID, userLite.ID, 0, callerID); errMedia == nil {
				avatarView = view
			}
		}

		requests = append(requests, auth_models.UserLiteView{
			User:     userLite,
			Avatar:   avatarView,
			IsOnline: cache_service.IsUserOnline(ctx, userLite.ID),
		})
	}

	// Prévention du `null` en JSON
	if requests == nil {
		requests = make([]auth_models.UserLiteView, 0)
	}

	return conversation_models.GetCommunityRequestsOutput{Requests: requests}, nil
}
