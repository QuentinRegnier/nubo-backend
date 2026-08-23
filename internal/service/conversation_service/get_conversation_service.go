package conversation_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// GetUserConversationPaginated récupère la liste des conversations avec une cascade complète L1 -> L2 -> L3
func GetUserConversationPaginated(ctx context.Context, callerID int64, input conversation_models.GetConversationInput) (conversation_models.GetInboxOutput, error) {
	var inboxItems []cache_service.InboxItemView

	// 0. GESTION DU MODE FORCE (Contournement et purge du L1 via DDD)
	if input.Force {
		_ = cache_service.PurgeUserConversations(ctx, callerID)
	}

	// 1. TENTATIVE L1 (SPEED CACHE) - Uniquement si on ne dépasse pas la limite RAM (100 max) et pas en Force
	if input.Offset < 100 && !input.Force {
		items, err := cache_service.GetInboxView(ctx, callerID, input.Limit, input.Offset)
		if err == nil && len(items) > 0 {
			inboxItems = items
		}
	}

	// 2. FALLBACK L2 (MONGO)
	if len(inboxItems) == 0 {
		mongoResults, err := mongo.MongoLoadConversationPaginated(callerID, input.Limit, input.Offset)
		if err == nil && len(mongoResults) > 0 {
			for _, res := range mongoResults {
				inboxItems = append(inboxItems, cache_service.InboxItemView{
					Conversation: models.ConvLiteRequest{
						ID:            res.Conversation.ID,
						Type:          res.Conversation.Type,
						Title:         res.Conversation.Title,
						LastMessageID: res.Conversation.LastMessageID,
					},
					Member: models.MemberLiteRequest{
						ConversationID: res.Member.ConversationID,
						UserID:         res.Member.UserID,
						Role:           res.Member.Role,
						UnreadCount:    res.Member.UnreadCount,
						JoinedAt:       res.Member.JoinedAt.UnixMilli(), // <-- AJOUT ICI
					},
				})

				// RÉHYDRATATION : Hit Mongo (L2) -> Réhydrate Redis (L1) !
				go func(fConv conversation_models.ConversationPayload, fMem conversation_models.MemberPayload, o int64) {
					bgCtx := context.Background()

					// 1. Réhydratation de l'Object Cache (Full Payloads)
					_ = object_cache_service.SetConversationInObjectCache(bgCtx, fConv)
					_ = object_cache_service.SetMemberInObjectCache(bgCtx, fMem)

					// 2. Réhydratation du Speed Cache (Lite Payloads & ZSET)
					cache_service.RehydrateConversationItemInSpeedCache(bgCtx, fConv, fMem, o)
				}(res.Conversation, res.Member, input.Offset)
			}
		}
	}

	// 3. FALLBACK ABSOLU L3 (POSTGRES)
	if len(inboxItems) == 0 {
		pgResults, err := postgres.FuncLoadConversationPaginated(ctx, callerID, input.Limit, input.Offset)
		if err == nil {
			for _, res := range pgResults {

				inboxItems = append(inboxItems, cache_service.InboxItemView{
					Conversation: models.ConvLiteRequest{
						ID:            res.Conversation.ID,
						Type:          res.Conversation.Type,
						Title:         res.Conversation.Title,
						LastMessageID: res.Conversation.LastMessageID,
					},
					Member: models.MemberLiteRequest{
						ConversationID: res.Member.ConversationID,
						UserID:         res.Member.UserID,
						Role:           res.Member.Role,
						UnreadCount:    res.Member.UnreadCount,
						JoinedAt:       res.Member.JoinedAt.UnixMilli(), // <-- AJOUT ICI
					},
				})

				// PROMOTION L3 -> L2 (Mongo) -> L1 (Redis)
				go func(fConv conversation_models.ConversationPayload, fMem conversation_models.MemberPayload, o int64) {
					bgCtx := context.Background()

					// A. Réhydratation L2 (MongoDB) avec les FULL Payloads
					_ = mongo.MongoUpsertConversation(fConv)
					_ = mongo.MongoUpsertMember(fMem)

					// B. Réhydratation L1 (OBJECT CACHE) avec les FULL Payloads
					_ = object_cache_service.SetConversationInObjectCache(bgCtx, fConv)
					_ = object_cache_service.SetMemberInObjectCache(bgCtx, fMem)

					// C. Réhydratation L1 (SPEED CACHE) avec le parsing interne Full->Lite
					cache_service.RehydrateConversationItemInSpeedCache(bgCtx, fConv, fMem, o)
				}(res.Conversation, res.Member, input.Offset)
			}
		}
	}

	// 4. MAPPING FINAL VERS L'API
	var result []conversation_models.InboxConversationView
	for _, item := range inboxItems {

		// ==== CALCUL DYNAMIQUE DES AVATARS ====
		avatars := GetConversationAvatars(ctx, item.Conversation.ID, callerID, item.Conversation.Type)

		view := conversation_models.InboxConversationView{
			ConversationID: item.Conversation.ID,
			Type:           item.Conversation.Type,
			Title:          item.Conversation.Title,
			LastMessageID:  item.Conversation.LastMessageID,
			Role:           item.Member.Role,
			UnreadCount:    item.Member.UnreadCount,
			Avatars:        avatars, // <-- INJECTION ICI
		}
		result = append(result, view)
	}

	// Renvoie un tableau vide plutôt que 'null' en JSON si pas de conversation
	if result == nil {
		result = make([]conversation_models.InboxConversationView, 0)
	}

	return conversation_models.GetInboxOutput{Conversations: result}, nil
}
