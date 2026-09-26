package cache_service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/member_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
	"github.com/vmihailenco/msgpack/v5"
)

// ############################################################################
// # DTOs ET MODÈLES DU SPEED CACHE CONVERSATIONNEL
// ############################################################################

// InboxItemView est la structure consolidée renvoyée à l'API pour l'Inbox.
type InboxItemView struct {
	Conversation lite_models.ConvLiteRequest   `json:"conversation"`
	Member       lite_models.MemberLiteRequest `json:"member"`
}

// ############################################################################
// # SERVICE : LECTURE ET ASSEMBLAGE DE L'INBOX HYBRIDE (L1 -> L3)
// ############################################################################

// GetInboxView assemble l'Inbox hybride (SPEED Cache L1 -> Postgres L3).
func GetInboxView(ctx context.Context, userID int64, paginationLimit int64, paginationOffset int64) ([]InboxItemView, error) {

	// ── ÉTAPE 1 : LECTURE DU ZSET INBOX (L1) ────────────────────────────────
	conversationIDStringsList, errRedisInbox := redis.UserInbox.ZRevRange(ctx, userID, paginationOffset, paginationOffset+paginationLimit-1)
	if errRedisInbox != nil {
		logger.Log.Error().Err(errRedisInbox).Int64("user_id", userID).Msg("Erreur L1 lors de la lecture du UserInbox ZSET")
		return nil, nubo_error.NewInternal()
	}

	if len(conversationIDStringsList) == 0 {
		return []InboxItemView{}, nil
	}

	var extractedConversationIDs []int64
	for _, idString := range conversationIDStringsList {
		if parsedID, errParse := strconv.ParseInt(idString, 10, 64); errParse == nil {
			extractedConversationIDs = append(extractedConversationIDs, parsedID)
		}
	}

	// ── ÉTAPE 2 : MGET MASSIF SUR LES MÉTADONNÉES DE CONVERSATION (CONVMETA) ─
	metaGetResult, errMetaGet := redis.ConvMeta.GetMany(ctx, extractedConversationIDs)
	var missingConversationIDs []int64

	if errMetaGet != nil {
		missingConversationIDs = extractedConversationIDs // Tout est manquant
		metaGetResult = &redis.GetManyResult{Found: make(map[int64][]byte)}
	} else {
		for _, missingID := range metaGetResult.MissingIDs {
			missingConversationIDs = append(missingConversationIDs, missingID)
		}
	}

	// ── ÉTAPE 3 : MGET MASSIF SUR LES MEMBRES (CONVMEMBERS) ─────────────────
	var memberCompositeKeysList []any
	for _, conversationID := range extractedConversationIDs {
		// Clé composite encapsulée (ex: "convID:userID")
		memberCompositeKeysList = append(memberCompositeKeysList, fmt.Sprintf("%d:%d", conversationID, userID))
	}

	memberValuesList, _ := redis.ConvMembers.MGet(ctx, memberCompositeKeysList...)

	foundMetasMap := make(map[int64]lite_models.ConvLiteRequest)
	foundMembersMap := make(map[int64]lite_models.MemberLiteRequest)
	var missingMemberIDs []int64

	// Désérialisation des métadonnées de conversations
	for conversationID, binaryData := range metaGetResult.Found {
		var metaPayload lite_models.ConvLiteRequest
		if errUnmarshal := msgpack.Unmarshal(binaryData, &metaPayload); errUnmarshal == nil {
			foundMetasMap[conversationID] = metaPayload
		} else {
			missingConversationIDs = append(missingConversationIDs, conversationID)
		}
	}

	// Désérialisation des objets Membres
	for index, rawMemberValue := range memberValuesList {
		currentConvID := extractedConversationIDs[index]

		if rawMemberValue != nil {
			if stringValue, ok := rawMemberValue.(string); ok {
				var memberPayload lite_models.MemberLiteRequest
				if errUnmarshal := msgpack.Unmarshal([]byte(stringValue), &memberPayload); errUnmarshal == nil {
					foundMembersMap[currentConvID] = memberPayload
					continue
				}
			}
		}
		missingMemberIDs = append(missingMemberIDs, currentConvID)
	}

	// ── ÉTAPE 4 : FALLBACK POSTGRES (AUTO-GUÉRISON SI DONNÉES MANQUANTES) ────
	uniquesMissingMap := make(map[int64]bool)
	for _, missingID := range missingConversationIDs {
		uniquesMissingMap[missingID] = true
	}
	for _, missingID := range missingMemberIDs {
		uniquesMissingMap[missingID] = true
	}

	var aggregatedMissingIDs []int64
	for missingID := range uniquesMissingMap {
		aggregatedMissingIDs = append(aggregatedMissingIDs, missingID)
	}

	if len(aggregatedMissingIDs) > 0 {
		logger.Log.Info().Int("missing_conversations", len(aggregatedMissingIDs)).Msg("Postgres Fallback déclenché pour l'Inbox (Cache Miss)")

		fallbackResultsFromPg, errPgFallback := postgres.FuncLoadConversationFallback(ctx, userID, aggregatedMissingIDs)
		if errPgFallback == nil {
			for _, fallbackRecord := range fallbackResultsFromPg {

				convLiteRequest := lite_models.ConvLiteRequest{
					ID:            fallbackRecord.Conversation.ID,
					Type:          fallbackRecord.Conversation.Type,
					Title:         fallbackRecord.Conversation.Title,
					Description:   fallbackRecord.Conversation.Description,
					AvatarID:      fallbackRecord.Conversation.AvatarID,
					LastMessageID: fallbackRecord.Conversation.LastMessageID,
					Settings:      fallbackRecord.Conversation.Settings,
					ExternalLink:  fallbackRecord.Conversation.ExternalLink,
				}

				memberLiteRequest := lite_models.MemberLiteRequest{
					ConversationID:    fallbackRecord.Member.ConversationID,
					UserID:            userID,
					Role:              fallbackRecord.Member.Role,
					Settings:          fallbackRecord.Member.Settings,
					FrozenMessageID:   fallbackRecord.Member.FrozenMessageID,
					LastReadMessageID: fallbackRecord.Member.LastReadMessageID,
					UnreadCount:       fallbackRecord.Member.UnreadCount,
					JoinedAt:          fallbackRecord.Member.JoinedAt,
				}

				foundMetasMap[fallbackRecord.Conversation.ID] = convLiteRequest
				foundMembersMap[fallbackRecord.Conversation.ID] = memberLiteRequest

				// AUTO-GUÉRISON SYNCHRONE ET ASYNCHRONE L3 -> L1
				go func(meta lite_models.ConvLiteRequest, mem lite_models.MemberLiteRequest) {
					backgroundCtx := context.Background()
					_ = redis.ConvMeta.SetObject(backgroundCtx, meta.ID, meta)

					compositeKey := fmt.Sprintf("%d:%d", mem.ConversationID, mem.UserID)
					_ = redis.ConvMembers.SetObject(backgroundCtx, compositeKey, mem)
				}(convLiteRequest, memberLiteRequest)
			}
		} else {
			logger.Log.Error().Err(errPgFallback).Msg("Échec critique lors du Fallback Postgres pour l'Inbox")
		}
	}

	// ── ÉTAPE 5 : ASSEMBLAGE FINAL ET TRI DYNAMIQUE ( ÉPINGLES / PINNED ) ───
	var finalInboxItemsList []InboxItemView
	for _, conversationID := range extractedConversationIDs {
		metaPayload, okMeta := foundMetasMap[conversationID]
		memberPayload, okMember := foundMembersMap[conversationID]

		if okMeta && okMember {
			finalInboxItemsList = append(finalInboxItemsList, InboxItemView{
				Conversation: metaPayload,
				Member:       memberPayload,
			})
		}
	}

	// Tri dynamique pour remonter les conversations épinglées (Pinned != -1) en haut de la liste
	sort.Slice(finalInboxItemsList, func(i, j int) bool {
		pinIndexI := finalInboxItemsList[i].Member.Settings.Pinned
		pinIndexJ := finalInboxItemsList[j].Member.Settings.Pinned

		// 1. Les deux sont épinglés (>= 0) -> tri par la valeur de l'index de pin
		if pinIndexI != -1 && pinIndexJ != -1 {
			return pinIndexI < pinIndexJ
		}
		// 2. Uniquement i est épinglé
		if pinIndexI != -1 {
			return true
		}
		// 3. Uniquement j est épinglé
		if pinIndexJ != -1 {
			return false
		}
		// 4. Aucun n'est épinglé (-1) -> tri chronologique inversé (Dernier message reçu en premier)
		return finalInboxItemsList[i].Conversation.LastMessageID > finalInboxItemsList[j].Conversation.LastMessageID
	})

	return finalInboxItemsList, nil
}

// ############################################################################
// # OPÉRATIONS D'ÉCRITURE (DÉLÉGATION DEPUIS LES WORKERS)
// ############################################################################

// ProcessNewMessageInSpeedCache met à jour les métadonnées, les compteurs non-lus
// et le classement Inbox, avec Auto-Guérison L2/L3 -> L1.
func ProcessNewMessageInSpeedCache(ctx context.Context, messageID int64, conversationID int64, senderID int64) ([]int64, error) {
	var notifiedUserIDs []int64

	// 1. AUTO-GUÉRISON DE LA CONVERSATION (CONVMETA)
	var convLiteRequest lite_models.ConvLiteRequest
	errMeta := redis.ConvMeta.GetObject(ctx, conversationID, &convLiteRequest)

	if errMeta != nil || convLiteRequest.ID == 0 {
		// Cache Miss : La conversation a été évincée de la RAM, réhydratation via Mongo (L2)
		mongoConv, errMongo := mongo.MongoGetConversation(conversationID)
		if errMongo == nil && mongoConv.ID != 0 {
			convLiteRequest = lite_models.ConvLiteRequest{
				ID:            mongoConv.ID,
				Type:          mongoConv.Type,
				Title:         mongoConv.Title,
				Description:   mongoConv.Description,
				AvatarID:      mongoConv.AvatarID,
				LastMessageID: messageID,
				Settings:      service.ToConversationSettingsLite(mongoConv.Settings),
				ExternalLink:  mongoConv.ExternalLink,
			}
			_ = redis.ConvMeta.SetObject(ctx, conversationID, convLiteRequest)
		}
	} else {
		convLiteRequest.LastMessageID = messageID
		_ = redis.ConvMeta.SetObject(ctx, conversationID, convLiteRequest)
	}

	// 2. RÉCUPÉRATION ET AUTO-GUÉRISON DES PARTICIPANTS
	participantStringsList, errRedisParts := redis.ConvParticipants.SMembers(ctx, conversationID)

	if errRedisParts != nil || len(participantStringsList) == 0 {
		postgresParticipantIDs, errPg := postgres.FuncGetConversationParticipantIDs(ctx, conversationID)
		if errPg == nil {
			for _, participantID := range postgresParticipantIDs {
				participantStringsList = append(participantStringsList, strconv.FormatInt(participantID, 10))
				_ = redis.ConvParticipants.SAdd(ctx, conversationID, participantID) // Réhydrate le SET L1
			}
		}
	}

	// 3. INCRÉMENTATION DES "UNREAD" ET MISE À JOUR DE L'INBOX
	for _, participantString := range participantStringsList {
		currentParticipantID, _ := strconv.ParseInt(participantString, 10, 64)

		// A. Mise à jour de l'Inbox ZSET pour TOUS les participants (remonte la conv en haut)
		_ = redis.UserInbox.ZAddWithCap(ctx, currentParticipantID, float64(messageID), conversationID, variables.MaxZsetInbox)
		_ = redis.InboxActivity.SetPrimitive(ctx, currentParticipantID, time.Now().UnixMilli())

		// B. Incrémentation du compteur de non-lus (Unread) pour les destinataires uniquement
		if currentParticipantID != senderID {
			var memberLiteRequest lite_models.MemberLiteRequest
			compositeMemberKey := fmt.Sprintf("%d:%d", conversationID, currentParticipantID)
			errMemberGet := redis.ConvMembers.GetObject(ctx, compositeMemberKey, &memberLiteRequest)

			if errMemberGet != nil || memberLiteRequest.ConversationID == 0 {
				// Cache Miss Membre : Réhydratation depuis Postgres (L3)
				pgMemberRecord, errPgMem := postgres.FuncGetMember(ctx, conversationID, currentParticipantID)
				if errPgMem == nil && pgMemberRecord.ID != 0 {
					memberLiteRequest = lite_models.MemberLiteRequest{
						ConversationID:    pgMemberRecord.ConversationID,
						UserID:            pgMemberRecord.UserID,
						Role:              pgMemberRecord.Role,
						Settings:          service.ToMemberSettingsLite(pgMemberRecord.Settings),
						FrozenMessageID:   pgMemberRecord.FrozenMessageID,
						LastReadMessageID: pgMemberRecord.LastReadMessageID,
						UnreadCount:       pgMemberRecord.UnreadCount + 1,
						JoinedAt:          pgMemberRecord.JoinedAt,
					}
					_ = redis.ConvMembers.SetObject(ctx, compositeMemberKey, memberLiteRequest)
				}
			} else {
				memberLiteRequest.UnreadCount++
				_ = redis.ConvMembers.SetObject(ctx, compositeMemberKey, memberLiteRequest)
			}

			notifiedUserIDs = append(notifiedUserIDs, currentParticipantID)
		}
	}

	return notifiedUserIDs, nil
}

// AddMemberToSpeedCache indexe un nouveau membre dans le cache de messagerie L1.
func AddMemberToSpeedCache(ctx context.Context, memberPayload lite_models.MemberLiteRequest) error {
	_ = redis.ConvParticipants.SAdd(ctx, memberPayload.ConversationID, memberPayload.UserID)
	compositeMemberKey := fmt.Sprintf("%d:%d", memberPayload.ConversationID, memberPayload.UserID)

	if memberPayload.Role >= 0 {
		UpdateCommunityMemberCountInSpeedCache(ctx, memberPayload.ConversationID, 1)
	}

	errSet := redis.ConvMembers.SetObject(ctx, compositeMemberKey, memberPayload)
	if errSet != nil {
		logger.Log.Error().Err(errSet).Msg("Impossible d'ajouter le membre dans le Speed Cache")
		return nubo_error.NewInternal()
	}

	return nil
}

// RemoveMemberFromSpeedCache nettoie les index lorsqu'un membre quitte ou est expulsé.
func RemoveMemberFromSpeedCache(ctx context.Context, conversationID int64, userID int64) error {
	compositeMemberKey := fmt.Sprintf("%d:%d", conversationID, userID)

	var oldMemberPayload lite_models.MemberLiteRequest
	if errGet := redis.ConvMembers.GetObject(ctx, compositeMemberKey, &oldMemberPayload); errGet == nil && oldMemberPayload.Role >= 0 {
		UpdateCommunityMemberCountInSpeedCache(ctx, conversationID, -1)
	}

	_ = redis.ConvParticipants.SRem(ctx, conversationID, userID)
	_ = redis.ConvMembers.DeleteObject(ctx, compositeMemberKey)

	errRem := redis.UserInbox.ZRem(ctx, userID, strconv.FormatInt(conversationID, 10))
	_ = redis.InboxActivity.SetPrimitive(ctx, userID, time.Now().UnixMilli())

	if errRem != nil {
		return nubo_error.NewInternal()
	}

	return nil
}

// ResetMemberUnreadCountInSpeedCache remet le compteur à 0 et met à jour le watermark de lecture.
func ResetMemberUnreadCountInSpeedCache(ctx context.Context, conversationID int64, userID int64, lastReadMessageID int64) error {
	var memberLitePayload lite_models.MemberLiteRequest
	compositeMemberKey := fmt.Sprintf("%d:%d", conversationID, userID)

	errGet := redis.ConvMembers.GetObject(ctx, compositeMemberKey, &memberLitePayload)
	if errGet == nil && memberLitePayload.ConversationID != 0 {
		memberLitePayload.UnreadCount = 0

		if lastReadMessageID > memberLitePayload.LastReadMessageID {
			memberLitePayload.LastReadMessageID = lastReadMessageID
		}

		errSet := redis.ConvMembers.SetObject(ctx, compositeMemberKey, memberLitePayload)
		if errSet != nil {
			return nubo_error.NewInternal()
		}

		_ = redis.InboxActivity.SetPrimitive(ctx, userID, time.Now().UnixMilli())
	}

	return nil
}

// ############################################################################
// # GESTION DU CYCLE DE VIE ET RÉHYDRATATION
// #################################No citations format validation#########

// PurgeUserConversations vide le ZSET inbox utilisateur pour forcer un rechargement L2/L3.
func PurgeUserConversations(ctx context.Context, userID int64) error {
	errDel := redis.UserInbox.DeleteObject(ctx, userID)
	if errDel != nil {
		return nubo_error.NewInternal()
	}
	return nil
}

// RehydrateConversationItemInSpeedCache réhydrate le cache L1 à partir d'une donnée fraîche L2/L3.
func RehydrateConversationItemInSpeedCache(ctx context.Context, fullConversation conversation_models.ConversationPayload, fullMember member_models.MemberPayload, paginationOffset int64) {
	if paginationOffset >= 100 {
		return
	}

	convLiteRequest := lite_models.ConvLiteRequest{
		ID:            fullConversation.ID,
		Type:          fullConversation.Type,
		Title:         fullConversation.Title,
		Description:   fullConversation.Description,
		AvatarID:      fullConversation.AvatarID,
		LastMessageID: fullConversation.LastMessageID,
		Settings:      service.ToConversationSettingsLite(fullConversation.Settings),
		ExternalLink:  fullConversation.ExternalLink,
	}

	memberLiteRequest := lite_models.MemberLiteRequest{
		ConversationID:    fullMember.ConversationID,
		UserID:            fullMember.UserID,
		Role:              fullMember.Role,
		Settings:          service.ToMemberSettingsLite(fullMember.Settings),
		FrozenMessageID:   fullMember.FrozenMessageID,
		LastReadMessageID: fullMember.LastReadMessageID,
		UnreadCount:       fullMember.UnreadCount,
		JoinedAt:          fullMember.JoinedAt,
	}

	_ = redis.ConvMeta.SetObject(ctx, convLiteRequest.ID, convLiteRequest)

	compositeMemberKey := fmt.Sprintf("%d:%d", memberLiteRequest.ConversationID, memberLiteRequest.UserID)
	_ = redis.ConvMembers.SetObject(ctx, compositeMemberKey, memberLiteRequest)

	_ = redis.ConvParticipants.SAdd(ctx, memberLiteRequest.ConversationID, memberLiteRequest.UserID)

	if convLiteRequest.LastMessageID > 0 {
		_ = redis.UserInbox.ZAddWithCap(ctx, memberLiteRequest.UserID, float64(convLiteRequest.LastMessageID), convLiteRequest.ID, 100)
	}
}

// ############################################################################
// # AMORÇAGE (SEEDING) MESSAGERIE
// ############################################################################

// SeedMessagingSpeedCache reconstruit l'intégralité du cache Inbox et Conversations depuis Postgres.
func SeedMessagingSpeedCache(ctx context.Context) error {
	logger.Log.Info().Msg("Amorçage SPEED Cache: Chargement de la messagerie (Conversations et Inboxes)...")

	activeConversationsList, errPgConv := postgres.FuncLoadActiveConversations(ctx)
	if errPgConv != nil {
		return nubo_error.NewInternal()
	}

	for _, conversationRecord := range activeConversationsList {
		convLiteRequest := lite_models.ConvLiteRequest{
			ID:            conversationRecord.ID,
			Type:          conversationRecord.Type,
			Title:         conversationRecord.Title,
			Description:   conversationRecord.Description,
			AvatarID:      conversationRecord.AvatarID,
			LastMessageID: conversationRecord.LastMessageID,
			Settings:      conversationRecord.Settings,
			ExternalLink:  conversationRecord.ExternalLink,
		}
		_ = redis.ConvMeta.SetObject(ctx, convLiteRequest.ID, convLiteRequest)
	}

	activeMembersList, errPgMem := postgres.FuncLoadActiveMembers(ctx)
	if errPgMem != nil {
		return nubo_error.NewInternal()
	}

	for _, activeMemberRecord := range activeMembersList {
		compositeMemberKey := fmt.Sprintf("%d:%d", activeMemberRecord.Member.ConversationID, activeMemberRecord.Member.UserID)

		memberLiteRequest := lite_models.MemberLiteRequest{
			ConversationID:    activeMemberRecord.Member.ConversationID,
			UserID:            activeMemberRecord.Member.UserID,
			Role:              activeMemberRecord.Member.Role,
			Settings:          activeMemberRecord.Member.Settings,
			FrozenMessageID:   activeMemberRecord.Member.FrozenMessageID,
			LastReadMessageID: activeMemberRecord.Member.LastReadMessageID,
			UnreadCount:       activeMemberRecord.Member.UnreadCount,
			JoinedAt:          activeMemberRecord.Member.JoinedAt,
		}

		_ = redis.ConvMembers.SetObject(ctx, compositeMemberKey, memberLiteRequest)
		_ = redis.ConvParticipants.SAdd(ctx, activeMemberRecord.Member.ConversationID, activeMemberRecord.Member.UserID)

		if activeMemberRecord.LastMessageID.Valid && activeMemberRecord.LastMessageID.Int64 > 0 {
			_ = redis.UserInbox.ZAddWithCap(ctx, activeMemberRecord.Member.UserID, float64(activeMemberRecord.LastMessageID.Int64), activeMemberRecord.Member.ConversationID, 100)
		}
	}

	logger.Log.Info().Int("conversations", len(activeConversationsList)).Int("membres", len(activeMembersList)).Msg("SPEED Cache Messaging initialisé avec succès.")
	return nil
}

// GetDirectConversationCache cherche l'existence d'une conversation privée en L1.
func GetDirectConversationCache(ctx context.Context, userOneID int64, userTwoID int64) (int64, error) {
	conversationIDStringsList, errRedis := redis.UserInbox.ZRevRange(ctx, userOneID, 0, -1)
	if errRedis != nil || len(conversationIDStringsList) == 0 {
		return 0, errors.New("cache miss")
	}

	var candidateConversationIDs []int64
	for _, idString := range conversationIDStringsList {
		if parsedID, errParse := strconv.ParseInt(idString, 10, 64); errParse == nil {
			candidateConversationIDs = append(candidateConversationIDs, parsedID)
		}
	}

	metaGetResult, errMetaGet := redis.ConvMeta.GetMany(ctx, candidateConversationIDs)
	if errMetaGet != nil {
		return 0, errors.New("cache miss")
	}

	userTwoStringID := strconv.FormatInt(userTwoID, 10)

	for _, conversationID := range candidateConversationIDs {
		binaryData, isFound := metaGetResult.Found[conversationID]
		if !isFound {
			continue
		}

		var metaPayload lite_models.ConvLiteRequest
		// Type 0 correspond aux messages privés (DM)
		if errUnmarshal := msgpack.Unmarshal(binaryData, &metaPayload); errUnmarshal == nil && metaPayload.Type == 0 {
			participantStringsList, errParts := redis.ConvParticipants.SMembers(ctx, conversationID)
			if errParts == nil {
				for _, participantString := range participantStringsList {
					if participantString == userTwoStringID {
						return conversationID, nil
					}
				}
			}
		}
	}

	return 0, errors.New("not found in speed cache")
}

// UpdateMemberSpeedCache écrase l'état complet du membre en RAM (O(1))
// et gère dynamiquement sa présence dans les listes de Fan-Out.
func UpdateMemberSpeedCache(ctx context.Context, memberPayload lite_models.MemberLiteRequest) error {
	compositeMemberKey := fmt.Sprintf("%d:%d", memberPayload.ConversationID, memberPayload.UserID)

	var oldMemberPayload lite_models.MemberLiteRequest
	wasActive := false
	if errGet := redis.ConvMembers.GetObject(ctx, compositeMemberKey, &oldMemberPayload); errGet == nil && oldMemberPayload.ConversationID != 0 {
		wasActive = oldMemberPayload.Role >= 0
	}
	isActive := memberPayload.Role >= 0

	if isActive && !wasActive {
		UpdateCommunityMemberCountInSpeedCache(ctx, memberPayload.ConversationID, 1)
	} else if !isActive && wasActive {
		UpdateCommunityMemberCountInSpeedCache(ctx, memberPayload.ConversationID, -1)
	}

	errSet := redis.ConvMembers.SetObject(ctx, compositeMemberKey, memberPayload)
	if errSet != nil {
		return nubo_error.NewInternal()
	}

	if memberPayload.Role < 0 {
		_ = redis.ConvParticipants.SRem(ctx, memberPayload.ConversationID, memberPayload.UserID)
	} else {
		_ = redis.ConvParticipants.SAdd(ctx, memberPayload.ConversationID, memberPayload.UserID)
		_ = redis.InboxActivity.SetPrimitive(ctx, memberPayload.UserID, time.Now().UnixMilli())
	}

	return nil
}

// AddConversationToUserInbox ajoute une conversation dans la boîte de réception d'un utilisateur.
func AddConversationToUserInbox(ctx context.Context, userID int64, conversationID int64, lastMessageID int64) error {
	errZAdd := redis.UserInbox.ZAddWithCap(ctx, userID, float64(lastMessageID), conversationID, 100)
	if errZAdd != nil {
		return nubo_error.NewInternal()
	}
	return nil
}

// TouchInboxActivity signale une modification dans la boîte de réception de l'utilisateur.
func TouchInboxActivity(ctx context.Context, userID int64) int64 {
	currentTimestampMs := time.Now().UnixMilli()
	_ = redis.InboxActivity.SetPrimitive(ctx, userID, currentTimestampMs)

	return currentTimestampMs
}
