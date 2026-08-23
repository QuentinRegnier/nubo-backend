package cache_service

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/vmihailenco/msgpack/v5"
)

// InboxItemView est la structure consolidée renvoyée à l'API
type InboxItemView struct {
	Conversation models.ConvLiteRequest   `json:"conversation"`
	Member       models.MemberLiteRequest `json:"member"`
}

// GetInboxView assemble l'Inbox hybride (SPEED Cache L1 -> Postgres L3)
func GetInboxView(ctx context.Context, userID int64, limit int64, offset int64) ([]InboxItemView, error) {
	// 1. Lire le ZSET Inbox en fonction de l'offset et de la limite
	convIDStrings, err := redis.UserInbox.ZRevRange(ctx, userID, offset, offset+limit-1)
	if err != nil {
		return nil, err
	}

	if len(convIDStrings) == 0 {
		return []InboxItemView{}, nil
	}

	var convIDs []int64
	for _, idStr := range convIDStrings {
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			convIDs = append(convIDs, id)
		}
	}

	// 2. MGET Massif sur ConvMeta
	metaRes, err := redis.ConvMeta.GetMany(ctx, convIDs)
	var missingConvIDs []int64
	if err != nil {
		missingConvIDs = convIDs // Tout manque
		metaRes = &redis.GetManyResult{Found: make(map[int64][]byte)}
	} else {
		missingConvIDs = metaRes.MissingIDs
	}

	// 3. MGET Massif sur ConvMembers (Clé composite encapsulée via Collection L1)
	var memberIDs []any
	for _, cid := range convIDs {
		// ID composite pur, le Repository s'occupera du préfixe
		memberIDs = append(memberIDs, fmt.Sprintf("%d:%d", cid, userID))
	}
	memberValues, _ := redis.ConvMembers.MGet(ctx, memberIDs...)

	foundMetas := make(map[int64]models.ConvLiteRequest)
	foundMembers := make(map[int64]models.MemberLiteRequest)
	var missingMemberIDs []int64

	// Désérialisation Metas
	for cid, data := range metaRes.Found {
		var meta models.ConvLiteRequest
		if err := msgpack.Unmarshal(data, &meta); err == nil {
			foundMetas[cid] = meta
		} else {
			missingConvIDs = append(missingConvIDs, cid)
		}
	}

	// Désérialisation Members
	for i, val := range memberValues {
		cid := convIDs[i]
		if val != nil {
			if strVal, ok := val.(string); ok {
				var mem models.MemberLiteRequest
				if err := msgpack.Unmarshal([]byte(strVal), &mem); err == nil {
					foundMembers[cid] = mem
					continue
				}
			}
		}
		missingMemberIDs = append(missingMemberIDs, cid)
	}

	// 4. FALLBACK POSTGRES (Auto-guérison si données manquantes via le Repository)
	missingMap := make(map[int64]bool)
	for _, id := range missingConvIDs {
		missingMap[id] = true
	}
	for _, id := range missingMemberIDs {
		missingMap[id] = true
	}

	var missingArray []int64
	for id := range missingMap {
		missingArray = append(missingArray, id)
	}

	if len(missingArray) > 0 {
		log.Printf("🛡️ Postgres Fallback déclenché pour %d conversations manquantes dans l'Inbox", len(missingArray))

		// Appel abstrait pur DDD : Zéro SQL dans le Cache Service !
		fallbackResults, err := postgres.FuncLoadConversationFallback(ctx, userID, missingArray)
		if err == nil {
			for _, res := range fallbackResults {
				foundMetas[res.Conversation.ID] = res.Conversation
				foundMembers[res.Conversation.ID] = res.Member

				// ⬆️ PROMOTION L3 -> L1 (Auto-Guérison du Cache)
				go func(mMeta models.ConvLiteRequest, mMem models.MemberLiteRequest) {
					bgCtx := context.Background()
					_ = redis.ConvMeta.SetObject(bgCtx, mMeta.ID, mMeta)
					memberID := fmt.Sprintf("%d:%d", mMem.ConversationID, mMem.UserID)
					_ = redis.ConvMembers.SetObject(bgCtx, memberID, mMem)
				}(res.Conversation, res.Member)
			}
		} else {
			log.Printf("⚠️ Erreur Fallback Postgres Inbox: %v", err)
		}
	}

	// 5. ASSEMBLAGE FINAL
	var finalInbox []InboxItemView
	for _, cid := range convIDs {
		meta, okMeta := foundMetas[cid]
		mem, okMem := foundMembers[cid]
		if okMeta && okMem {
			finalInbox = append(finalInbox, InboxItemView{
				Conversation: meta,
				Member:       mem,
			})
		}
	}

	return finalInbox, nil
}

// ============================================================================
// OPÉRATIONS D'ÉCRITURE (Délégation depuis les Workers)
// ============================================================================

// ProcessNewMessageInSpeedCache met à jour les métadonnées, les compteurs non-lus et le classement Inbox,
// avec Auto-Guérison L2/L3 -> L1 en cas de Cache Miss.
// Retourne la liste des UserIDs qui ont reçu un +1 pour le compteur asynchrone.
func ProcessNewMessageInSpeedCache(ctx context.Context, msgID int64, convID int64, senderID int64) ([]int64, error) {
	var updatedUsers []int64

	// 1. AUTO-GUÉRISON DE LA CONVERSATION (ConvMeta)
	var convLite models.ConvLiteRequest
	errMeta := redis.ConvMeta.GetObject(ctx, convID, &convLite)
	if errMeta != nil || convLite.ID == 0 {
		// CACHE MISS : La conversation a été évincée par Redis (volatile-lfu). On réhydrate via Mongo (L2).
		if mongoConv, errMongo := mongo.MongoGetConversation(convID); errMongo == nil && mongoConv.ID != 0 {
			convLite = models.ConvLiteRequest{
				ID:            mongoConv.ID,
				Type:          mongoConv.Type,
				Title:         mongoConv.Title,
				LastMessageID: msgID, // On injecte directement le nouveau message
			}
			_ = redis.ConvMeta.SetObject(ctx, convID, convLite)
		}
	} else {
		// HIT : Mise à jour classique
		convLite.LastMessageID = msgID
		_ = redis.ConvMeta.SetObject(ctx, convID, convLite)
	}

	// 2. RÉCUPÉRATION ET MISE À JOUR DES PARTICIPANTS
	participants, err := redis.ConvParticipants.SMembers(ctx, convID)

	// AUTO-GUÉRISON DES PARTICIPANTS
	if err != nil || len(participants) == 0 {
		// DÉLÉGATION DDD STRICTE : Le Cache Service ne parle qu'au Repository Postgres
		pgIDs, errPg := postgres.FuncGetConversationParticipantIDs(ctx, convID)
		if errPg == nil {
			for _, pID := range pgIDs {
				participants = append(participants, strconv.FormatInt(pID, 10))
				_ = redis.ConvParticipants.SAdd(ctx, convID, pID) // Réhydrate le SET L1
			}
		}
	}

	// 3. INCRÉMENTATION DES "UNREAD" ET MISE À JOUR INBOX
	for _, participantStr := range participants {
		participantID, _ := strconv.ParseInt(participantStr, 10, 64)

		// 1. Mettre à jour l'Inbox ZSET de TOUS les participants (y compris l'expéditeur)
		// -> La conversation remonte tout en haut pour tout le monde instantanément.
		_ = redis.UserInbox.ZAddWithCap(ctx, participantID, float64(msgID), convID, 100)
		// NOUVEAU : On signale de l'activité sur la boite aux lettres de CE participant
		_ = redis.InboxActivity.SetPrimitive(ctx, participantID, time.Now().UnixMilli())

		// 2. Incrémenter unread_count pour les destinataires uniquement (Sauf l'expéditeur)
		if participantID != senderID {
			var memberLite models.MemberLiteRequest
			memberID := fmt.Sprintf("%d:%d", convID, participantID)
			errMem := redis.ConvMembers.GetObject(ctx, memberID, &memberLite)

			if errMem != nil || memberLite.ConversationID == 0 {
				// CACHE MISS MEMBER : On réhydrate l'objet Member complet depuis L2/L3
				if pgMem, errPg := postgres.FuncGetMember(ctx, convID, participantID); errPg == nil && pgMem.ID != 0 {
					memberLite = models.MemberLiteRequest{
						ConversationID: pgMem.ConversationID,
						UserID:         pgMem.UserID,
						Role:           pgMem.Role,
						UnreadCount:    pgMem.UnreadCount + 1, // On ajoute le nouveau message
						JoinedAt:       pgMem.JoinedAt.UnixMilli(),
					}
					_ = redis.ConvMembers.SetObject(ctx, memberID, memberLite)
				}
			} else {
				// HIT : Mise à jour classique
				memberLite.UnreadCount++
				_ = redis.ConvMembers.SetObject(ctx, memberID, memberLite)
			}

			// On ajoute l'utilisateur à la liste des compteurs à envoyer au Worker
			updatedUsers = append(updatedUsers, participantID)
		}
	}

	return updatedUsers, nil
}

// AddMemberToSpeedCache indexe un nouveau membre dans le cache de messagerie
func AddMemberToSpeedCache(ctx context.Context, member models.MemberLiteRequest) error {
	_ = redis.ConvParticipants.SAdd(ctx, member.ConversationID, member.UserID)
	memberID := fmt.Sprintf("%d:%d", member.ConversationID, member.UserID)
	return redis.ConvMembers.SetObject(ctx, memberID, member)
}

// RemoveMemberFromSpeedCache nettoie les index lorsqu'un membre quitte ou est expulsé
func RemoveMemberFromSpeedCache(ctx context.Context, convID int64, userID int64) error {
	_ = redis.ConvParticipants.SRem(ctx, convID, userID)
	memberID := fmt.Sprintf("%d:%d", convID, userID)
	_ = redis.ConvMembers.DeleteObject(ctx, memberID)

	err := redis.UserInbox.ZRem(ctx, userID, strconv.FormatInt(convID, 10))
	_ = redis.InboxActivity.SetPrimitive(ctx, userID, time.Now().UnixMilli()) // NOUVEAU
	return err
}

// ResetMemberUnreadCountInSpeedCache remet le compteur de messages non lus à 0 pour un membre (Mode DDD)
func ResetMemberUnreadCountInSpeedCache(ctx context.Context, convID int64, userID int64) error {
	var memberLite models.MemberLiteRequest
	memberID := fmt.Sprintf("%d:%d", convID, userID)

	// Si le membre est en Speed Cache, on le met à jour
	if err := redis.ConvMembers.GetObject(ctx, memberID, &memberLite); err == nil {
		memberLite.UnreadCount = 0
		errSet := redis.ConvMembers.SetObject(ctx, memberID, memberLite)
		_ = redis.InboxActivity.SetPrimitive(ctx, userID, time.Now().UnixMilli()) // NOUVEAU
		return errSet
	}
	return nil // Ne pas faire d'erreur si l'utilisateur n'a pas chargé cette info en RAM récemment
}

// ============================================================================
// GESTION DU CYCLE DE VIE ET RÉHYDRATATION (DDD)
// ============================================================================

// PurgeUserConversations vide le ZSET de la boîte de réception pour forcer un rechargement L2/L3 (Mode Force)
func PurgeUserConversations(ctx context.Context, userID int64) error {
	return redis.UserInbox.DeleteObject(ctx, userID)
}

// RehydrateConversationItemInSpeedCache réhydrate le cache L1 à partir d'une donnée fraîche L2/L3 (Format Complet)
func RehydrateConversationItemInSpeedCache(ctx context.Context, fullConv conversation_models.ConversationPayload, fullMem conversation_models.MemberPayload, offset int64) {
	// Sécurité RAM : on ne réhydrate le ZSET et les hash que si on est dans le Scope du Speed Cache
	if offset >= 100 {
		return
	}

	// Parsing en dur (Full -> Lite)
	convLite := models.ConvLiteRequest{
		ID:            fullConv.ID,
		Type:          fullConv.Type,
		Title:         fullConv.Title,
		LastMessageID: fullConv.LastMessageID,
	}

	memLite := models.MemberLiteRequest{
		ConversationID: fullMem.ConversationID,
		UserID:         fullMem.UserID,
		Role:           fullMem.Role,
		UnreadCount:    fullMem.UnreadCount,
		JoinedAt:       fullMem.JoinedAt.UnixMilli(),
	}

	// 1. Restauration de la Méta (O(1))
	_ = redis.ConvMeta.SetObject(ctx, convLite.ID, convLite)

	// 2. Restauration du Membre (O(1))
	memberID := fmt.Sprintf("%d:%d", memLite.ConversationID, memLite.UserID)
	_ = redis.ConvMembers.SetObject(ctx, memberID, memLite)

	// 3. Restauration des Participants (Set de distribution pour le Fan-Out)
	_ = redis.ConvParticipants.SAdd(ctx, memLite.ConversationID, memLite.UserID)

	// 4. Restauration du ZSET Inbox de l'utilisateur
	if convLite.LastMessageID > 0 {
		_ = redis.UserInbox.ZAddWithCap(ctx, memLite.UserID, float64(convLite.LastMessageID), convLite.ID, 100)
	}
}

// ============================================================================
// AMORÇAGE (SEEDING)
// ============================================================================

// SeedMessagingSpeedCache reconstruit l'intégralité du cache Inbox et Conversations depuis Postgres.
// Appelé uniquement lors d'un "Cold Start" de Redis.
func SeedMessagingSpeedCache(ctx context.Context) error {
	log.Println("  Amorçage SPEED Cache: Chargement des Conversations et Inboxes...")

	// 1. Récupération des Conversations Actives via Repository (DDD Pur)
	conversations, err := postgres.FuncLoadActiveConversations(ctx)
	if err != nil {
		return fmt.Errorf("erreur DB seed conversations: %w", err)
	}

	for _, conv := range conversations {
		_ = redis.ConvMeta.SetObject(ctx, conv.ID, conv)
	}

	// 2. Récupération des Membres et Hydratation du ZSET Inbox
	activeMembers, err := postgres.FuncLoadActiveMembers(ctx)
	if err != nil {
		return fmt.Errorf("erreur DB seed members: %w", err)
	}

	for _, activeMem := range activeMembers {
		// A. Remplissage ConvMembers (Object Cache)
		memberID := fmt.Sprintf("%d:%d", activeMem.Member.ConversationID, activeMem.Member.UserID)
		_ = redis.ConvMembers.SetObject(ctx, memberID, activeMem.Member)

		// B. Remplissage ConvParticipants (Set de distribution)
		_ = redis.ConvParticipants.SAdd(ctx, activeMem.Member.ConversationID, activeMem.Member.UserID)

		// C. Remplissage Inbox de l'utilisateur (ZSET Chronologique)
		// Uniquement si la conversation a déjà un message actif (Sinon elle polluerait le ZSET avec un score 0)
		if activeMem.LastMessageID.Valid && activeMem.LastMessageID.Int64 > 0 {
			_ = redis.UserInbox.ZAddWithCap(ctx, activeMem.Member.UserID, float64(activeMem.LastMessageID.Int64), activeMem.Member.ConversationID, 100)
		}
	}

	log.Printf("  SPEED Cache Messaging terminées (%d convos, %d membres chargés).", len(conversations), len(activeMembers))
	return nil
}

// GetDirectConversationCache cherche l'existence d'une conversation privée en L1.
// O(n) ultra-rapide sur le ZSET de l'inbox de u1, avec filtrage sur ConvMeta et ConvParticipants.
func GetDirectConversationCache(ctx context.Context, u1, u2 int64) (int64, error) {
	// 1. Récupération de tous les IDs de conversation de l'utilisateur
	convIDStrings, err := redis.UserInbox.ZRevRange(ctx, u1, 0, -1)
	if err != nil || len(convIDStrings) == 0 {
		return 0, fmt.Errorf("cache miss")
	}

	var convIDs []int64
	for _, idStr := range convIDStrings {
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			convIDs = append(convIDs, id)
		}
	}

	// 2. Filtrage du Type en O(1) via MGET
	metaRes, err := redis.ConvMeta.GetMany(ctx, convIDs)
	if err != nil {
		return 0, fmt.Errorf("cache miss")
	}

	u2Str := strconv.FormatInt(u2, 10)

	// 3. Boucle de validation
	for _, cid := range convIDs {
		data, ok := metaRes.Found[cid]
		if !ok {
			continue
		}

		var meta models.ConvLiteRequest
		if err := msgpack.Unmarshal(data, &meta); err == nil && meta.Type == 0 {
			// C'est un Message Privé, on vérifie si l'autre utilisateur est dedans
			participants, errPart := redis.ConvParticipants.SMembers(ctx, cid)
			if errPart == nil {
				for _, p := range participants {
					if p == u2Str {
						return cid, nil // Trouvé !
					}
				}
			}
		}
	}

	return 0, fmt.Errorf("not found in speed cache")
}

// UpdateMemberStateInSpeedCache écrase l'état complet du membre en RAM (O(1))
// et gère dynamiquement sa présence dans les listes de Fan-Out.
func UpdateMemberSpeedCache(ctx context.Context, member models.MemberLiteRequest) error {
	memberID := fmt.Sprintf("%d:%d", member.ConversationID, member.UserID)

	// 1. Écrasement total avec le payload frais (Zéro cherry-picking)
	_ = redis.ConvMembers.SetObject(ctx, memberID, member)

	// 2. Gestion autonome du Fan-Out
	if member.Role < 0 {
		_ = redis.ConvParticipants.SRem(ctx, member.ConversationID, member.UserID)
	} else {
		_ = redis.ConvParticipants.SAdd(ctx, member.ConversationID, member.UserID)
	}
	_ = redis.InboxActivity.SetPrimitive(ctx, member.UserID, time.Now().UnixMilli()) // NOUVEAU
	return nil
}

// AddConversationToUserInbox ajoute (ou remonte) une conversation dans la boîte de réception d'un utilisateur
func AddConversationToUserInbox(ctx context.Context, userID int64, convID int64, lastMessageID int64) error {
	return redis.UserInbox.ZAddWithCap(ctx, userID, float64(lastMessageID), convID, 100)
}
