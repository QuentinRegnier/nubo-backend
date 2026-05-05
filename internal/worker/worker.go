package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
)

// --- CONFIGURATION DU CERVEAU (Modifiable via .env) ---
var (
	MaxBatchSize int64
	MinBackoff   time.Duration
	MaxBackoff   time.Duration
)

func init() {
	// 1. Taille du Batch (Ex: 5000)
	if val, err := strconv.ParseInt(os.Getenv("WORKER_MAX_BATCH_SIZE"), 10, 64); err == nil && val > 0 {
		MaxBatchSize = val
	} else {
		MaxBatchSize = 5000 // Valeur par défaut
	}

	// 2. Backoff Minimum (Période d'hyperactivité, ex: 50ms)
	if val, err := strconv.Atoi(os.Getenv("WORKER_MIN_BACKOFF_MS")); err == nil && val > 0 {
		MinBackoff = time.Duration(val) * time.Millisecond
	} else {
		MinBackoff = 50 * time.Millisecond // Valeur par défaut
	}

	// 3. Backoff Maximum (Sommeil profond, ex: 1000ms)
	if val, err := strconv.Atoi(os.Getenv("WORKER_MAX_BACKOFF_MS")); err == nil && val > 0 {
		MaxBackoff = time.Duration(val) * time.Millisecond
	} else {
		MaxBackoff = 1 * time.Second // Valeur par défaut
	}
}

func runWorker(ctx context.Context, shardID int) {
	currentBackoff := MinBackoff

	for {
		// 1. Vérification de l'arrêt gracieux du serveur
		select {
		case <-ctx.Done():
			return
		default:
		}

		// 2. Blocage absolu (0 CPU) via BLMPOP / BLPOP
		// On limite la taille via MaxBatchSize (dynamique)
		events, err := redis.PopSmartBatchBlocking(ctx, shardID, MaxBatchSize)

		if err != nil {
			log.Printf("⚠️ Worker %d: Erreur Redis: %v", shardID, err)
			time.Sleep(1 * time.Second) // Protection anti-boucle infinie si Redis crashe
			continue
		}

		// 3. Traitement dynamique
		if len(events) > 0 {
			processBatch(ctx, events)
			// RESET DU SOMMEIL : on a trouvé du travail, on repasse à la vitesse max !
			currentBackoff = MinBackoff
		} else {
			// SLEEP : la file était vide (malgré le blocage), on s'endort doucement
			time.Sleep(currentBackoff)
			currentBackoff *= 2
			if currentBackoff > MaxBackoff {
				currentBackoff = MaxBackoff
			}
		}
	}
}

// processBatch trie les événements et les envoie aux bases ET au cache
func processBatch(ctx context.Context, events []redis.AsyncEvent) {
	var mongoEvents []redis.AsyncEvent
	var pgEvents []redis.AsyncEvent

	for _, evt := range events {
		if evt.Targets&redis.TargetMongo != 0 {
			mongoEvents = append(mongoEvents, evt)
		}
		if evt.Targets&redis.TargetPostgres != 0 {
			pgEvents = append(pgEvents, evt)
		}
	}

	// Étape 1 : Exécution Parallèle des bases de données (Mongo & Postgres)
	done := make(chan bool)

	go func() {
		if len(mongoEvents) > 0 {
			flushMongo(ctx, mongoEvents)
		}
		done <- true
	}()

	go func() {
		if len(pgEvents) > 0 {
			flushPostgres(ctx, pgEvents)
		}
		done <- true
	}()

	// On DOIT attendre que la BDD ait validé les transactions sur le disque
	// avant de mettre à jour le cache, sinon on lira des valeurs périmées.
	<-done
	<-done

	// Étape 2 : Mise à jour de l'index de découverte (MOST Cache)
	// S'exécute de manière asynchrone mais séquencée APRÈS la BDD.
	updateMostCache(ctx, events)
}

// updateMostCache intercepte les événements pour alimenter les ZSETs (Tags, Profils, Classements)
func updateMostCache(ctx context.Context, events []redis.AsyncEvent) {
	for _, e := range events {

		// --- SPEED CACHE : NOUVEL UTILISATEUR ---
		// Interception de la création d'utilisateur pour l'auto-complétion et le Registre
		if e.Type == redis.EntityUser && e.Action == redis.ActionCreate {
			jsonBytes, err := json.Marshal(e.Payload)
			if err == nil {
				var user domain.UserRequest
				if err := json.Unmarshal(jsonBytes, &user); err == nil {
					// Action 1 : Créer le UserLite
					userLite := domain.UserLiteRequest{
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
					var convLite domain.ConvLiteRequest
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
								var memberLite domain.MemberLiteRequest
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
				var member domain.MemberLiteRequest
				if err := json.Unmarshal(jsonBytes, &member); err == nil && member.ConversationID != 0 {

					// Action A : Ajouter l'ID au SET Redis des participants de cette conversation
					participantsKey := fmt.Sprintf("conv:participants:%d", member.ConversationID)
					_ = redisgo.Rdb.SAdd(ctx, participantsKey, member.UserID).Err()

					// Action B : Initialiser son profil MemberLite dans le cache
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

					// Action B : Nettoyer son MemberLite du cache
					memberID := fmt.Sprintf("%d:%d", member.ConversationID, member.UserID)
					_ = redis.ConvMembers.DeleteObject(ctx, memberID)

					// Action C : Supprimer la conversation de son Inbox (ZSET)
					inboxKey := fmt.Sprintf("inbox:user:%d", member.UserID)
					_ = redisgo.Rdb.ZRem(ctx, inboxKey, member.ConversationID).Err()
				}
			}
		}

		// 1. SI C'EST UN NOUVEAU POST
		if e.Type == redis.EntityPost && e.Action == redis.ActionCreate {
			jsonBytes, err := json.Marshal(e.Payload)
			if err == nil {
				var post domain.PostRequest
				if err := json.Unmarshal(jsonBytes, &post); err == nil {
					// A. Algorithme de Recommandation (Tags, Global, Recent)
					service.UpdatePostRecommendationScore(ctx, post.ID, post.Hashtags)
					// B. Chronologie Utilisateur (Grille Profil) avec précision temporelle stricte
					service.AddPostToUserProfile(ctx, post.UserID, post.ID, post.CreatedAt.UnixMilli())
					// C. Vecteur de Contenu pour Recommandation Personnalisée (Pilier 3)
					service.StoreContentVector(ctx, post)
				}
			}
		}

		// 2. SI C'EST UNE SUPPRESSION DE POST (Cache Busting)
		if e.Type == redis.EntityPost && e.Action == redis.ActionDelete {
			jsonBytes, err := json.Marshal(e.Payload)
			if err == nil {
				var post domain.PostRequest
				// On s'assure d'avoir bien pu extraire le UserID du payload de suppression
				if err := json.Unmarshal(jsonBytes, &post); err == nil && post.UserID != 0 {
					// Invalidation radicale : on détruit le ZSET de l'utilisateur.
					// Zéro dérive d'état garantie.
					service.InvalidateUserProfileCache(ctx, post.UserID)
				}
			}
		}

		// 3. SI C'EST UNE INTERACTION (LIKE ou VUE agrégé)
		if (e.Type == redis.EntityLike || e.Type == redis.EntityView) && e.Action == redis.ActionCreate {
			jsonBytes, err := json.Marshal(e.Payload)
			if err == nil {
				// STRUCTURE COMMUNE : Intègre le count et le drapeau d'idempotence
				var interactionEvent struct {
					TargetID              int64 `json:"target_id"`
					Count                 int   `json:"count"`
					AlreadyEvaluatedRedis bool  `json:"already_evaluated_redis"`
				}

				if err := json.Unmarshal(jsonBytes, &interactionEvent); err == nil && interactionEvent.TargetID != 0 {

					// À ce stade, flushPostgres a déjà écrit les nouveaux compteurs en base.
					// 1. On détruit le cache L1 obsolète pour forcer un rafraîchissement
					// (L'interaction n'a pas mis à jour le cache local pour éviter les Race Conditions)
					_ = redis.Posts.DeleteObject(ctx, interactionEvent.TargetID)

					// 2. On utilise notre pipeline Dataloader (L3 Postgres -> L1 Redis)
					// pour récupérer l'entité avec ses valeurs parfaitement exactes et la remettre en RAM
					posts, err := service.GetPostsView([]int64{interactionEvent.TargetID})
					if err == nil && len(posts) > 0 {
						p := posts[0]

						// 3. On route vers les fonctions strictes qui vont :
						//    - Mettre à jour les classements absolus (rank:likes:strict)
						//    - Appeler le moteur de Time-Decay avec les nouveaux compteurs
						if e.Type == redis.EntityLike {
							service.EvaluatePostAfterLike(ctx, p.ID, float64(p.LikeCount), p.Hashtags)
						} else if e.Type == redis.EntityView {
							service.EvaluatePostAfterView(ctx, p.ID, float64(p.ViewCount), p.Hashtags)
						}
					}
				}
			}
		}
	}
}
