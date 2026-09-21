package object_cache_service

import (
	"context"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/member_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// GetMemberFromObjectCache récupère le payload complet d'un membre depuis le cache LFU
func GetMemberFromObjectCache(ctx context.Context, convID int64, userID int64) (member_models.MemberPayload, error) {
	var m member_models.MemberPayload
	// Utilisation d'une clé composite
	memberID := fmt.Sprintf("%d:%d", convID, userID)
	err := redis.Members.GetObject(ctx, memberID, &m)
	return m, err
}

// SetMemberInObjectCache insère ou met à jour un membre dans le cache LFU
func SetMemberInObjectCache(ctx context.Context, member member_models.MemberPayload) error {
	memberID := fmt.Sprintf("%d:%d", member.ConversationID, member.UserID)
	return redis.Members.SetObject(ctx, memberID, member)
}

// DeleteMemberFromObjectCache supprime un membre du cache LFU
func DeleteMemberFromObjectCache(ctx context.Context, convID int64, userID int64) error {
	memberID := fmt.Sprintf("%d:%d", convID, userID)
	return redis.Members.DeleteObject(ctx, memberID)
}
