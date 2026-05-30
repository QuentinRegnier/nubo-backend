package cache

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/lib/pq"
	"github.com/vmihailenco/msgpack/v5"
)

// InboxItemView est la structure consolidée renvoyée à l'API
type InboxItemView struct {
	Conversation domain.ConvLiteRequest   `json:"conversation"`
	Member       domain.MemberLiteRequest `json:"member"`
}

// GetInboxView assemble l'Inbox hybride (SPEED Cache L1 -> Postgres L2)
func GetInboxView(ctx context.Context, userID int64) ([]InboxItemView, error) {
	inboxKey := fmt.Sprintf("inbox:user:%d", userID)

	// 1. Lire le ZSET Inbox (les 50 conversations les plus récentes)
	convIDStrings, err := redis.ZRevRange(ctx, inboxKey, 0, 49)
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

	// 3. MGET Massif sur ConvMembers (Clé composite string)
	var memberKeys []string
	for _, cid := range convIDs {
		memberKeys = append(memberKeys, redis.ConvMembers.Key(fmt.Sprintf("%d:%d", cid, userID)))
	}

	memberValues, _ := redisgo.Rdb.MGet(ctx, memberKeys...).Result()

	foundMetas := make(map[int64]domain.ConvLiteRequest)
	foundMembers := make(map[int64]domain.MemberLiteRequest)
	var missingMemberIDs []int64

	// Désérialisation Metas
	for cid, data := range metaRes.Found {
		var meta domain.ConvLiteRequest
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
				var mem domain.MemberLiteRequest
				if err := msgpack.Unmarshal([]byte(strVal), &mem); err == nil {
					foundMembers[cid] = mem
					continue
				}
			}
		}
		missingMemberIDs = append(missingMemberIDs, cid)
	}

	// 4. FALLBACK POSTGRES (Auto-guérison si données manquantes)
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

		query := `
			SELECT conversation_id, title, type, last_message_id, role, unread_count 
			FROM messaging.func_load_conversation($1) 
			WHERE conversation_id = ANY($2)
		`

		rows, err := postgres.PostgresDB.QueryContext(ctx, query, userID, pq.Array(missingArray))
		if err == nil {
			defer func(rows *sql.Rows) {
				err := rows.Close()
				if err != nil {
					log.Printf("⚠️ Erreur lors de la fermeture des lignes Postgres: %v", err)
				}
			}(rows)
			for rows.Next() {
				var cid int64
				var title sql.NullString
				var cType int
				var lastMsgID sql.NullInt64
				var role, unreadCount int

				if err := rows.Scan(&cid, &title, &cType, &lastMsgID, &role, &unreadCount); err == nil {
					// Reconstruction ConvLite
					meta := domain.ConvLiteRequest{
						ID:   cid,
						Type: cType,
					}
					if title.Valid {
						meta.Title = title.String
					}
					if lastMsgID.Valid {
						meta.LastMessageID = lastMsgID.Int64
					}
					foundMetas[cid] = meta

					// Reconstruction MemberLite
					mem := domain.MemberLiteRequest{
						ConversationID: cid,
						UserID:         userID,
						Role:           role,
						UnreadCount:    unreadCount,
					}
					foundMembers[cid] = mem

					// ⬆️ PROMOTION L2 -> L1 (Auto-Guérison du Cache)
					go func(mMeta domain.ConvLiteRequest, mMem domain.MemberLiteRequest) {
						bgCtx := context.Background()
						_ = redis.ConvMeta.SetObject(bgCtx, mMeta.ID, mMeta)
						memberID := fmt.Sprintf("%d:%d", mMem.ConversationID, mMem.UserID)
						_ = redis.ConvMembers.SetObject(bgCtx, memberID, mMem)
					}(meta, mem)
				}
			}
		} else {
			log.Printf("⚠️ Erreur Fallback Postgres Inbox: %v", err)
		}
	}

	// 5. ASSEMBLAGE FINAL
	var finalInbox []InboxItemView
	for _, cid := range convIDs {
		// On ne retourne que les éléments où les deux morceaux ont pu être récupérés
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
