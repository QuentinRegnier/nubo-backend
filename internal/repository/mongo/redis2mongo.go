package mongo

import (
	"errors"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// Redis2Mongo fait le pont entre le domaine (Redis EntityType) et les collections MongoDB.
func Redis2Mongo(entity redis.EntityType) (*MongoCollection, error) {
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
		return nil, nubo_error.NewInternal(errors.New("entité non supportée pour vérification Mongo"))
	}
}
