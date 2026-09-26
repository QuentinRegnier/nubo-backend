package conversation_service

import (
	"context"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/member_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # SERVICE : RÉCUPÉRATION DU ROSTER DES MEMBRES PAR CONVERSATION
// ############################################################################

// GetConversationMembers extrait les membres des conversations demandées,
// vérifie les droits d'accès et hydrate les profils avec auto-guérison L1 -> L2 -> L3.
func GetConversationMembers(ctx context.Context, callerID int64, input member_models.GetConversationMembersInput) ([]member_models.ConversationMembersList, error) {
	rosterResults := make([]member_models.ConversationMembersList, 0, len(input.ConversationIDs))

	for _, conversationID := range input.ConversationIDs {
		// ── ÉTAPE 1 : VÉRIFICATION D'APPARTENANCE (SÉCURITÉ) ──────────────────
		if _, errMembership := security_service.LeftMember(ctx, conversationID, callerID); errMembership != nil {
			continue // Exclusion silencieuse des conversations inaccessibles
		}

		// ── ÉTAPE 2 : IDENTIFICATION DU TYPE DE CONVERSATION ───────────────────
		conversationPayload, errConv := object_cache_service.GetConversationFromObjectCache(ctx, conversationID)
		if errConv != nil || conversationPayload.ID == 0 {
			conversationPayload, _ = mongo.MongoGetConversation(conversationID)
			if conversationPayload.ID == 0 {
				var errPg error
				conversationPayload, errPg = postgres.FuncGetConversation(ctx, conversationID)
				if errPg != nil {
					logger.Log.Warn().Err(errPg).Int64("conv_id", conversationID).Msg("Échec fallback Postgres pour conversation")
					continue
				}
			}
		}

		// ── ÉTAPE 3 : RÉCUPÉRATION DES IDENTIFIANTS DES PARTICIPANTS ───────────
		var participantIDs []int64
		cachedParticipantsList, errRedisMembers := redis.ConvParticipants.SMembers(ctx, conversationID)

		if errRedisMembers == nil && len(cachedParticipantsList) > 0 {
			for _, participantStr := range cachedParticipantsList {
				if parsedID, errParse := strconv.ParseInt(participantStr, 10, 64); errParse == nil {
					participantIDs = append(participantIDs, parsedID)
				}
			}
		} else {
			var errPgParticipants error
			participantIDs, errPgParticipants = postgres.FuncGetConversationParticipantIDs(ctx, conversationID)
			if errPgParticipants != nil {
				logger.Log.Error().Err(errPgParticipants).Int64("conv_id", conversationID).Msg("Erreur L3 lors du chargement des participants")
				return nil, nubo_error.NewInternal()
			}
			// Auto-guérison L1 du set de distribution
			for _, pID := range participantIDs {
				_ = redis.ConvParticipants.SAdd(ctx, conversationID, pID)
			}
		}

		// ── ÉTAPE 4 : HYDRATATION DU MODÈLE DE CHAQUE MEMBRE ───────────────────
		hydratedMembersView := make([]member_models.MemberView, 0, len(participantIDs))

		for _, participantUserID := range participantIDs {
			memberPayload, errCacheMember := object_cache_service.GetMemberFromObjectCache(ctx, conversationID, participantUserID)

			if errCacheMember != nil || memberPayload.ID == 0 {
				var errMongo error
				memberPayload, errMongo = mongo.MongoGetMember(conversationID, participantUserID)
				if errMongo == nil && memberPayload.ID != 0 {
					go func(m member_models.MemberPayload) {
						_ = object_cache_service.SetMemberInObjectCache(context.Background(), m)
					}(memberPayload)
				} else {
					var errPg error
					memberPayload, errPg = postgres.FuncGetMember(ctx, conversationID, participantUserID)
					if errPg != nil {
						logger.Log.Warn().Err(errPg).Int64("user_id", participantUserID).Msg("Échec fallback Postgres pour membre")
						continue
					}
					if memberPayload.ID != 0 {
						go func(m member_models.MemberPayload) {
							bgContext := context.Background()
							_ = object_cache_service.SetMemberInObjectCache(bgContext, m)
							_ = redis.EnqueueDB(bgContext, m.ID, m.ConversationID, redis.EntityMembers, redis.ActionUpdate, m, redis.TargetMongo)
						}(memberPayload)
					}
				}
			}

			// Exclusion des participants ayant quitté ou ayant été bannis
			if memberPayload.ID == 0 || memberPayload.Role < variables.MemberRoleNormal {
				continue
			}

			memberView := member_models.MemberView{
				MemberPayload: memberPayload,
				IsOnline:      cache_service.IsUserOnline(ctx, participantUserID),
			}

			if userLiteData, errLite := cache_service.GetUserLite(ctx, participantUserID); errLite == nil {
				memberView.Username = userLiteData.Username

				if conversationPayload.Type == variables.ConversationTypeCommunityPriv || conversationPayload.Type == variables.ConversationTypeCommunityPub {
					memberView.AvatarCommunityID = userLiteData.ProfilePictureID
				} else if userLiteData.ProfilePictureID > 0 {
					if avatarView, errMedia := media_service.GenerateMediaViewCascade(ctx, userLiteData.ProfilePictureID, participantUserID, 0, callerID); errMedia == nil {
						memberView.Avatar = avatarView
					}
				}
			}

			hydratedMembersView = append(hydratedMembersView, memberView)
		}

		rosterResults = append(rosterResults, member_models.ConversationMembersList{
			ConversationID: conversationID,
			Members:        hydratedMembersView,
		})
	}

	return rosterResults, nil
}
