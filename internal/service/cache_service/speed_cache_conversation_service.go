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
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/vmihailenco/msgpack/v5"
)

// InboxItemView est la structure consolidée renvoyée à l'API
type InboxItemView struct {
	Conversation lite_models.ConvLiteRequest   `json:"conversation"`
	Member       lite_models.MemberLiteRequest `json:"member"`
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

	foundMetas := make(map[int64]lite_models.ConvLiteRequest)
	foundMembers := make(map[int64]lite_models.MemberLiteRequest)
	var missingMemberIDs []int64

	// Désérialisation Metas
	for cid, data := range metaRes.Found {
		var meta lite_models.ConvLiteRequest
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
				var mem lite_models.MemberLiteRequest
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
		logger.Log.Info().Int("missing_conversations", len(missingArray)).Msg("Postgres Fallback déclenché pour l'Inbox")

		// Appel abstrait pur DDD : Zéro SQL dans le Cache Service !
		fallbackResults, err := postgres.FuncLoadConversationFallback(ctx, userID, missingArray)
		if err == nil {
			for _, res := range fallbackResults {

				// 🛠️ CORRECTION : Utilisation de la structure imbriquée
				convLite := lite_models.ConvLiteRequest{
					ID:            res.Conversation.ID,
					Type:          res.Conversation.Type,
					Title:         res.Conversation.Title,
					LastMessageID: res.Conversation.LastMessageID,
				}

				memLite := lite_models.MemberLiteRequest{
					ConversationID: res.Member.ConversationID,
					UserID:         userID, // On connaît le UserID puisqu'on l'a passé à la fonction
					Role:           res.Member.Role,
					Settings:       res.Member.Settings,
					UnreadCount:    res.Member.UnreadCount,
					JoinedAt:       res.Member.JoinedAt,
				}

				// ✅ ASSIGNATION SÉCURISÉE
				foundMetas[res.Conversation.ID] = convLite
				foundMembers[res.Conversation.ID] = memLite

				// ⬆️ PROMOTION L3 -> L1 (Auto-Guérison du Cache)
				go func(mMeta lite_models.ConvLiteRequest, mMem lite_models.MemberLiteRequest) {
					bgCtx := context.Background()
					_ = redis.ConvMeta.SetObject(bgCtx, mMeta.ID, mMeta)
					memberID := fmt.Sprintf("%d:%d", mMem.ConversationID, mMem.UserID)
					_ = redis.ConvMembers.SetObject(bgCtx, memberID, mMem)
				}(convLite, memLite)
			}
		} else {
			logger.Log.Error().Err(err).Msg("Erreur Fallback Postgres Inbox")
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

	// === NOUVEAU : TRI DYNAMIQUE AVEC IS_PINNED ===
	// On trie l'inbox finale pour remonter les épingles (Pinned != -1) en haut de la liste.
	sort.Slice(finalInbox, func(i, j int) bool {
		pinI := finalInbox[i].Member.Settings.Pinned
		pinJ := finalInbox[j].Member.Settings.Pinned

		// 1. Les deux sont épinglés (>= 0) -> on trie par la valeur de l'épingle (ex: 1 avant 2)
		if pinI != -1 && pinJ != -1 {
			return pinI < pinJ
		}
		// 2. Uniquement i est épinglé
		if pinI != -1 {
			return true
		}
		// 3. Uniquement j est épinglé
		if pinJ != -1 {
			return false
		}
		// 4. Aucun n'est épinglé (-1) -> on conserve le tri temporel (LastMessageID décroissant)
		return finalInbox[i].Conversation.LastMessageID > finalInbox[j].Conversation.LastMessageID
	})

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
	var convLite lite_models.ConvLiteRequest
	errMeta := redis.ConvMeta.GetObject(ctx, convID, &convLite)
	if errMeta != nil || convLite.ID == 0 {
		// CACHE MISS : La conversation a été évincée par Redis (volatile-lfu). On réhydrate via Mongo (L2).
		if mongoConv, errMongo := mongo.MongoGetConversation(convID); errMongo == nil && mongoConv.ID != 0 {
			convLite = lite_models.ConvLiteRequest{
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
			var memberLite lite_models.MemberLiteRequest
			memberID := fmt.Sprintf("%d:%d", convID, participantID)
			errMem := redis.ConvMembers.GetObject(ctx, memberID, &memberLite)

			if errMem != nil || memberLite.ConversationID == 0 {
				// CACHE MISS MEMBER : On réhydrate l'objet Member complet depuis L2/L3
				if pgMem, errPg := postgres.FuncGetMember(ctx, convID, participantID); errPg == nil && pgMem.ID != 0 {
					memberLite = lite_models.MemberLiteRequest{
						ConversationID: pgMem.ConversationID,
						UserID:         pgMem.UserID,
						Role:           pgMem.Role,
						Settings:       service.ToMemberSettingsLite(pgMem.Settings),
						UnreadCount:    pgMem.UnreadCount + 1, // On ajoute le nouveau message
						JoinedAt:       pgMem.JoinedAt,
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
func AddMemberToSpeedCache(ctx context.Context, member lite_models.MemberLiteRequest) error {
	_ = redis.ConvParticipants.SAdd(ctx, member.ConversationID, member.UserID)
	memberID := fmt.Sprintf("%d:%d", member.ConversationID, member.UserID)

	// NOUVEAU : On incrémente si le membre est actif (Role >= 0)
	if member.Role >= 0 {
		UpdateCommunityMemberCountInSpeedCache(ctx, member.ConversationID, 1)
	}

	return redis.ConvMembers.SetObject(ctx, memberID, member)
}

// RemoveMemberFromSpeedCache nettoie les index lorsqu'un membre quitte ou est expulsé
func RemoveMemberFromSpeedCache(ctx context.Context, convID int64, userID int64) error {
	memberID := fmt.Sprintf("%d:%d", convID, userID)

	// NOUVEAU : Décrémenter le compteur si le membre était actif
	var oldMember lite_models.MemberLiteRequest
	if err := redis.ConvMembers.GetObject(ctx, memberID, &oldMember); err == nil && oldMember.Role >= 0 {
		UpdateCommunityMemberCountInSpeedCache(ctx, convID, -1)
	}

	_ = redis.ConvParticipants.SRem(ctx, convID, userID)
	_ = redis.ConvMembers.DeleteObject(ctx, memberID)
	err := redis.UserInbox.ZRem(ctx, userID, strconv.FormatInt(convID, 10))
	_ = redis.InboxActivity.SetPrimitive(ctx, userID, time.Now().UnixMilli()) // NOUVEAU
	return err
}

// ResetMemberUnreadCountInSpeedCache remet le compteur de messages non lus à 0 pour un membre (Mode DDD)
func ResetMemberUnreadCountInSpeedCache(ctx context.Context, convID int64, userID int64) error {
	var memberLite lite_models.MemberLiteRequest
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
	convLite := lite_models.ConvLiteRequest{
		ID:            fullConv.ID,
		Type:          fullConv.Type,
		Title:         fullConv.Title,
		LastMessageID: fullConv.LastMessageID,
	}

	memLite := lite_models.MemberLiteRequest{
		ConversationID: fullMem.ConversationID,
		UserID:         fullMem.UserID,
		Role:           fullMem.Role,
		Settings:       service.ToMemberSettingsLite(fullMem.Settings),
		UnreadCount:    fullMem.UnreadCount,
		JoinedAt:       fullMem.JoinedAt,
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
	logger.Log.Info().Msg("Amorçage SPEED Cache: Chargement des Conversations et Inboxes...")

	// 1. Récupération des Conversations Actives via Repository (DDD Pur)
	conversations, err := postgres.FuncLoadActiveConversations(ctx)
	if err != nil {
		return nubo_error.NewInternal(err)
	}

	for _, conv := range conversations {
		// 🛠️ CORRECTION : Mapping vers ConvLiteRequest pour le SetObject
		convLite := lite_models.ConvLiteRequest{
			ID:            conv.ID,
			Type:          conv.Type,
			Title:         conv.Title,
			LastMessageID: conv.LastMessageID,
		}
		_ = redis.ConvMeta.SetObject(ctx, convLite.ID, convLite)
	}

	// 2. Récupération des Membres et Hydratation du ZSET Inbox
	activeMembers, err := postgres.FuncLoadActiveMembers(ctx)
	if err != nil {
		return nubo_error.NewInternal(err)
	}

	for _, activeMem := range activeMembers {
		// 🛠️ CORRECTION : activeMem est une structure plate. On crée le MemberLiteRequest.
		memberID := fmt.Sprintf("%d:%d", activeMem.Member.ConversationID, activeMem.Member.UserID)

		memLite := lite_models.MemberLiteRequest{
			ConversationID: activeMem.Member.ConversationID,
			UserID:         activeMem.Member.UserID,
			Role:           activeMem.Member.Role,
			Settings:       activeMem.Member.Settings,
			UnreadCount:    activeMem.Member.UnreadCount,
			JoinedAt:       activeMem.Member.JoinedAt,
		}

		// A. Remplissage ConvMembers (Object Cache)
		_ = redis.ConvMembers.SetObject(ctx, memberID, memLite)

		// B. Remplissage ConvParticipants (Set de distribution)
		_ = redis.ConvParticipants.SAdd(ctx, activeMem.Member.ConversationID, activeMem.Member.UserID)

		// C. Remplissage Inbox de l'utilisateur (ZSET Chronologique)
		// Uniquement si la conversation a déjà un message actif (Sinon elle polluerait le ZSET avec un score 0)
		if activeMem.LastMessageID.Valid && activeMem.LastMessageID.Int64 > 0 {
			_ = redis.UserInbox.ZAddWithCap(ctx, activeMem.Member.UserID, float64(activeMem.LastMessageID.Int64), activeMem.Member.ConversationID, 100)
		}
	}

	logger.Log.Info().Int("conversations", len(conversations)).Int("membres", len(activeMembers)).Msg("SPEED Cache Messaging terminées")
	return nil
}

// GetDirectConversationCache cherche l'existence d'une conversation privée en L1.
// O(n) ultra-rapide sur le ZSET de l'inbox de u1, avec filtrage sur ConvMeta et ConvParticipants.
func GetDirectConversationCache(ctx context.Context, u1, u2 int64) (int64, error) {
	// 1. Récupération de tous les IDs de conversation de l'utilisateur
	convIDStrings, err := redis.UserInbox.ZRevRange(ctx, u1, 0, -1)
	if err != nil || len(convIDStrings) == 0 {
		return 0, errors.New("cache miss") // C'est une sentinelle interne
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
		return 0, errors.New("cache miss") // C'est une sentinelle interne
	}

	u2Str := strconv.FormatInt(u2, 10)

	// 3. Boucle de validation
	for _, cid := range convIDs {
		data, ok := metaRes.Found[cid]
		if !ok {
			continue
		}

		var meta lite_models.ConvLiteRequest
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

	return 0, errors.New("not found in speed cache") // C'est une sentinelle interne
}

// UpdateMemberSpeedCache écrase l'état complet du membre en RAM (O(1))
// et gère dynamiquement sa présence dans les listes de Fan-Out.
func UpdateMemberSpeedCache(ctx context.Context, member lite_models.MemberLiteRequest) error {
	memberID := fmt.Sprintf("%d:%d", member.ConversationID, member.UserID)

	// NOUVEAU : Détecter si on passe d'actif à inactif ou inversement
	var oldMember lite_models.MemberLiteRequest
	wasActive := false
	if err := redis.ConvMembers.GetObject(ctx, memberID, &oldMember); err == nil && oldMember.ConversationID != 0 {
		wasActive = oldMember.Role >= 0
	}
	isActive := member.Role >= 0

	if isActive && !wasActive {
		UpdateCommunityMemberCountInSpeedCache(ctx, member.ConversationID, 1)
	} else if !isActive && wasActive {
		UpdateCommunityMemberCountInSpeedCache(ctx, member.ConversationID, -1)
	}

	// 1. Écrasement total avec le payload frais (Zéro cherry-picking)
	_ = redis.ConvMembers.SetObject(ctx, memberID, member)

	// 2. Gestion autonome du Fan-Out
	if member.Role < 0 {
		_ = redis.ConvParticipants.SRem(ctx, member.ConversationID, member.UserID)
	} else {
		_ = redis.ConvParticipants.SAdd(ctx, member.ConversationID, member.UserID)
		_ = redis.InboxActivity.SetPrimitive(ctx, member.UserID, time.Now().UnixMilli()) // NOUVEAU
	}
	return nil
}

// AddConversationToUserInbox ajoute (ou remonte) une conversation dans la boîte de réception d'un utilisateur
func AddConversationToUserInbox(ctx context.Context, userID int64, convID int64, lastMessageID int64) error {
	return redis.UserInbox.ZAddWithCap(ctx, userID, float64(lastMessageID), convID, 100)
}

// TouchInboxActivity signale une modification dans la boîte de réception de l'utilisateur
// et retourne le timestamp exact (en millisecondes) généré par le serveur.
func TouchInboxActivity(ctx context.Context, userID int64) int64 {
	nowMs := time.Now().UnixMilli()
	// On met à jour le cache L1 pour que le prochain /sync sache qu'il y a eu du mouvement
	_ = redis.InboxActivity.SetPrimitive(ctx, userID, nowMs)

	return nowMs
}
