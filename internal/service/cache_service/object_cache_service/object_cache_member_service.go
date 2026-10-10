package object_cache_service

import (
	"context"
	"fmt"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/member_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_log"
	"github.com/QuentinRegnier/numan-backend/internal/repository/redis"
)

// ############################################################################
// # SERVICE : OBJECT CACHE (MEMBRES LFU)
// ############################################################################

// GetMemberFromObjectCache récupère le payload complet d'un membre depuis le cache LFU.
func GetMemberFromObjectCache(ctx context.Context, conversationID int64, userID int64) (member_models.MemberPayload, error) {
	var memberPayload member_models.MemberPayload

	// Utilisation d'une clé composite encapsulée
	compositeMemberKey := fmt.Sprintf("%d:%d", conversationID, userID)

	errRedis := redis.Members.GetObject(ctx, compositeMemberKey, &memberPayload)
	if errRedis != nil {
		return member_models.MemberPayload{}, errRedis
	}

	return memberPayload, nil
}

// SetMemberInObjectCache insère ou met à jour un membre dans le cache LFU.
func SetMemberInObjectCache(ctx context.Context, memberPayload member_models.MemberPayload) error {
	compositeMemberKey := fmt.Sprintf("%d:%d", memberPayload.ConversationID, memberPayload.UserID)

	errRedis := redis.Members.SetObject(ctx, compositeMemberKey, memberPayload)
	if errRedis != nil {
		numan_log.Error(ctx).Err(errRedis).Int64("user_id", memberPayload.UserID).Msg("Échec de l'écriture du membre dans l'Object Cache")
		return numan_error.NewInternal()
	}

	return nil
}
