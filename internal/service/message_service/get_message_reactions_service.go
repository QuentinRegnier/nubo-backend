package message_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// GetMessageReactions exécute le Slow Path pour récupérer la liste paginée et détaillée des réacteurs.
func GetMessageReactions(ctx context.Context, callerID int64, input message_models.GetMessageReactionsInput) (message_models.GetMessageReactionsOutput, error) {
	// 1. SÉCURITÉ : L'utilisateur doit pouvoir voir ce message
	_, err := security_service.LeftMessage(ctx, input.MessageID, callerID)
	if err != nil {
		return message_models.GetMessageReactionsOutput{}, err
	}

	var reactions []message_models.MessageReactionPayload

	// 2. RÉCUPÉRATION L2 (MongoDB) PRIORITAIRE
	mongoReactions, errMongo := mongo.MongoGetMessageReactionsPaginated(input.MessageID, int64(input.Limit), int64(input.Offset))
	if errMongo == nil && len(mongoReactions) > 0 {
		reactions = mongoReactions
	} else {
		// 3. FALLBACK ABSOLU L3 (PostgreSQL)
		pgReactions, errPg := postgres.FuncGetMessageReactionsPaginated(ctx, input.MessageID, input.Limit, input.Offset)
		if errPg != nil {
			return message_models.GetMessageReactionsOutput{
				MessageID: input.MessageID,
				Reactions: []message_models.UserReactionView{},
			}, nil
		}
		reactions = pgReactions

		// ⬆️ AUTO-GUÉRISON L2 (Asynchrone)
		for _, r := range reactions {
			go func(react message_models.MessageReactionPayload) {
				bgCtx := context.Background()
				// EntityMessageReaction va déclencher l'UPSERT côté Worker Mongo (SetUpsert: true)
				_ = redis.EnqueueDB(bgCtx, react.ID, react.MessageID, redis.EntityMessageReaction, redis.ActionUpdate, react, redis.TargetMongo)
			}(r)
		}
	}

	// 4. HYDRATATION EN CASCADE (SPEED CACHE & DOMAINE MÉDIA)
	var userViews []message_models.UserReactionView

	for _, r := range reactions {
		// Extraction en O(1) du cache L1
		if userLite, errLite := cache_service.GetUserLite(ctx, r.UserID); errLite == nil {
			var avatarView media_models.MediaView

			if userLite.ProfilePictureID > 0 {
				// Signature HMAC sécurisée de l'avatar
				if view, errMedia := media_service.GenerateMediaViewCascade(ctx, userLite.ProfilePictureID, r.UserID, 0, callerID); errMedia == nil {
					avatarView = view
				}
			}

			userViews = append(userViews, message_models.UserReactionView{
				User: auth_models.UserLiteView{
					User:     userLite,
					Avatar:   avatarView,
					IsOnline: cache_service.IsUserOnline(ctx, r.UserID),
				},
				Reaction: r.Reaction,
			})
		}
	}

	// Prévention du retour JSON 'null'
	if userViews == nil {
		userViews = make([]message_models.UserReactionView, 0)
	}

	return message_models.GetMessageReactionsOutput{
		MessageID: input.MessageID,
		Reactions: userViews,
	}, nil
}
