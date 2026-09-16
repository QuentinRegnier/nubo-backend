package postgres

import (
	"errors"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

type PostgresTarget struct {
	Schema string
	Table  string
}

// Redis2Postgres fait le pont entre le domaine (Redis EntityType) et le schéma physique SQL.
func Redis2Postgres(entity redis.EntityType) (PostgresTarget, error) {
	switch entity {
	case redis.EntityUser:
		return PostgresTarget{Schema: "auth", Table: "users"}, nil
	case redis.EntityUserSettings:
		return PostgresTarget{Schema: "auth", Table: "user_settings"}, nil
	case redis.EntitySession:
		return PostgresTarget{Schema: "auth", Table: "sessions"}, nil
	case redis.EntityRelation:
		return PostgresTarget{Schema: "auth", Table: "relations"}, nil
	case redis.EntityPost:
		return PostgresTarget{Schema: "content", Table: "posts"}, nil
	case redis.EntityComment:
		return PostgresTarget{Schema: "content", Table: "comments"}, nil
	case redis.EntityLike:
		return PostgresTarget{Schema: "content", Table: "likes"}, nil
	case redis.EntityMedia:
		return PostgresTarget{Schema: "content", Table: "media"}, nil
	case redis.EntityConversation:
		return PostgresTarget{Schema: "messaging", Table: "conversations"}, nil
	case redis.EntityMembers:
		return PostgresTarget{Schema: "messaging", Table: "members"}, nil
	case redis.EntityMessage:
		return PostgresTarget{Schema: "messaging", Table: "messages"}, nil
	default:
		return PostgresTarget{}, nubo_error.NewInternal(errors.New("entité non supportée pour vérification Postgres"))
	}
}
