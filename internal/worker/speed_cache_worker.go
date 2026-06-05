package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

func updateSpeedCache(ctx context.Context, e redis.AsyncEvent) {
	// --- SPEED CACHE : NOUVEL UTILISATEUR ---
	// Interception de la création d'utilisateur pour l'auto-complétion et le Registre
	if e.Type == redis.EntityUser && e.Action == redis.ActionCreate {
		jsonBytes, err := json.Marshal(e.Payload)
		if err == nil {
			var user auth_models.UserPayload
			if err := json.Unmarshal(jsonBytes, &user); err == nil {
				// Action 1 : Créer le UserLite
				userLite := models.UserLiteRequest{
					ID:               user.ID,
					Username:         user.Username,
					FirstName:        user.FirstName,
					LastName:         user.LastName,
					ProfilePictureID: user.ProfilePictureID,
					Bio:              user.Bio,
					Grade:            user.Grade,
					Badges:           user.Badges,
				}

				// Sauvegarde de l'objet Lite dans Redis
				_ = redis.UsersLite.SetObject(ctx, userLite.ID, userLite)

				// Action 2 : Ajouter "pseudo:id" dans le ZSET lexicographique
				lexMember := fmt.Sprintf("%s:%d", strings.ToLower(userLite.Username), userLite.ID)
				_ = redis.ZAddLex(ctx, "users:search:lex", lexMember)
			}
		}
	}

	// --- SPEED CACHE : NOUVEAU MESSAGE ---
	// Interception pour mettre à jour l'Inbox et les compteurs
	if e.Type == redis.EntityMessage && e.Action == redis.ActionCreate {
		jsonBytes, err := json.Marshal(e.Payload)
		if err == nil {
			var msg struct {
				ID             int64 `json:"id"`
				ConversationID int64 `json:"conversation_id"`
				SenderID       int64 `json:"sender_id"`
			}
			if err := json.Unmarshal(jsonBytes, &msg); err == nil && msg.ID != 0 {

				// Action 1 : Mettre à jour last_message_id dans ConvMeta
				var convLite models.ConvLiteRequest
				if err := redis.ConvMeta.GetObject(ctx, msg.ConversationID, &convLite); err == nil {
					convLite.LastMessageID = msg.ID
					_ = redis.ConvMeta.SetObject(ctx, convLite.ID, convLite)
				}

				// Action 2 & 3 : Récupération ultra-rapide via Redis SET (Au revoir Postgres !)
				participantsKey := fmt.Sprintf("conv:participants:%d", msg.ConversationID)
				participants, err := redisgo.Rdb.SMembers(ctx, participantsKey).Result()

				if err == nil {
					for _, participantStr := range participants {
						participantID, _ := strconv.ParseInt(participantStr, 10, 64)

						// Action 2 : Incrémenter unread_count pour les destinataires (pas pour l'expéditeur)
						if participantID != msg.SenderID {
							var memberLite models.MemberLiteRequest
							// Clé composite : ID_Conversation:ID_User
							memberID := fmt.Sprintf("%d:%d", msg.ConversationID, participantID)

							if err := redis.ConvMembers.GetObject(ctx, memberID, &memberLite); err == nil {
								memberLite.UnreadCount++
								_ = redis.ConvMembers.SetObject(ctx, memberID, memberLite)
							}
						}

						// Action 3 : Mettre à jour l'Inbox ZSET de TOUS les participants
						inboxKey := fmt.Sprintf("inbox:user:%d", participantID)
						// Règle des 100 : on plafonne à 100 conversations par utilisateur
						_ = redis.ZAddWithCap(ctx, inboxKey, float64(msg.ID), msg.ConversationID, 100)
					}
				}
			}
		}
	}

	// ====================================================================
	// --- SPEED CACHE : CYCLE DE VIE DES PARTICIPANTS (LE SET REDIS) ---
	// ====================================================================

	// 1. NOUVEAU MEMBRE (Rejoint un groupe ou création d'une conversation)
	if e.Type == redis.EntityMembers && e.Action == redis.ActionCreate {
		jsonBytes, err := json.Marshal(e.Payload)
		if err == nil {
			var member models.MemberLiteRequest
			if err := json.Unmarshal(jsonBytes, &member); err == nil && member.ConversationID != 0 {

				// Action A : Ajouter l'ID au SET Redis des participants de cette conversation
				participantsKey := fmt.Sprintf("conv:participants:%d", member.ConversationID)
				_ = redisgo.Rdb.SAdd(ctx, participantsKey, member.UserID).Err()

				// Action B : Initialiser son profil MemberLite dans le cache_service
				memberID := fmt.Sprintf("%d:%d", member.ConversationID, member.UserID)
				_ = redis.ConvMembers.SetObject(ctx, memberID, member)

				// Note : On ne l'ajoute pas forcément à son ZSET Inbox tout de suite.
				// Il remontera tout seul dans son Inbox au premier message envoyé.
			}
		}
	}

	// 2. DÉPART D'UN MEMBRE (Quitte le groupe ou est expulsé)
	if e.Type == redis.EntityMembers && e.Action == redis.ActionDelete {
		jsonBytes, err := json.Marshal(e.Payload)
		if err == nil {
			// Souvent lors d'un Delete, le payload contient les clés d'identification
			var member struct {
				ConversationID int64 `json:"conversation_id"`
				UserID         int64 `json:"user_id"`
			}
			if err := json.Unmarshal(jsonBytes, &member); err == nil && member.ConversationID != 0 {

				// Action A : Retirer l'ID du SET Redis des participants
				participantsKey := fmt.Sprintf("conv:participants:%d", member.ConversationID)
				_ = redisgo.Rdb.SRem(ctx, participantsKey, member.UserID).Err()

				// Action B : Nettoyer son MemberLite du cache_service
				memberID := fmt.Sprintf("%d:%d", member.ConversationID, member.UserID)
				_ = redis.ConvMembers.DeleteObject(ctx, memberID)

				// Action C : Supprimer la conversation de son Inbox (ZSET)
				inboxKey := fmt.Sprintf("inbox:user:%d", member.UserID)
				_ = redisgo.Rdb.ZRem(ctx, inboxKey, member.ConversationID).Err()
			}
		}
	}
}
