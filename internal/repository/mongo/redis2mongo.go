package mongo

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_log"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// Redis2Mongo fait le pont entre le domaine (Redis EntityType) et les collections MongoDB.
func Redis2Mongo(ctx context.Context, entity redis.EntityType) (*Collection, error) {
	switch entity {
	case redis.EntityUser:
		return Users, nil
	case redis.EntityUserSettings:
		return UserSettings, nil
	case redis.EntitySession:
		return Sessions, nil
	case redis.EntityRelation:
		return Relations, nil
	case redis.EntityPost:
		return Posts, nil
	case redis.EntityComment:
		return Comments, nil
	case redis.EntityLike:
		return Likes, nil
	case redis.EntityMedia:
		return Media, nil
	case redis.EntityConversation:
		return Conversations, nil
	case redis.EntityMembers:
		return Members, nil
	case redis.EntityMessage:
		return Messages, nil
	default:
		nubo_log.Error(ctx).Str("entity", string(entity)).Msg("Entité non supportée pour la résolution de la collection MongoDB")
		return nil, nubo_error.NewInternal()
	}
}
