package tools

import (
	"reflect"
)

// ---------------- Schemas ----------------

// UsersCache représente la structure du cache "users"
var UsersSchema = map[string]reflect.Kind{
	"id":                 reflect.String, // UUID
	"username":           reflect.String,
	"email":              reflect.String,
	"email_verified":     reflect.Bool,
	"phone":              reflect.String,
	"phone_verified":     reflect.Bool,
	"password_hash":      reflect.String,
	"first_name":         reflect.String,
	"last_name":          reflect.String,
	"birthdate":          reflect.String, // Date au format string
	"sex":                reflect.Int,
	"bio":                reflect.String,
	"profile_picture_id": reflect.String, // UUID
	"grade":              reflect.Int,
	"location":           reflect.String,
	"school":             reflect.String,
	"work":               reflect.String,
	"badges":             reflect.Slice, // Text[]
	"desactivated":       reflect.Bool,
	"banned":             reflect.Bool,
	"ban_reason":         reflect.String,
	"ban_expires_at":     reflect.String, // Timestamp au format string
	"created_at":         reflect.String,
	"updated_at":         reflect.String,
}

// UserSettingsCache représente la structure du cache "user_settings"
var UserSettingsSchema = map[string]reflect.Kind{
	"id":            reflect.String,
	"user_id":       reflect.String,
	"privacy":       reflect.Map, // JSONB
	"notifications": reflect.Map, // JSONB
	"language":      reflect.String,
	"theme":         reflect.Int,
}

// SessionsCache
var SessionsSchema = map[string]reflect.Kind{
	"id":            reflect.String,
	"user_id":       reflect.String,
	"refresh_token": reflect.String,
	"device_info":   reflect.Map,    // JSONB
	"device_token":  reflect.String, // FCM token
	"ip_history":    reflect.Slice,  // INET[]
	"created_at":    reflect.String,
	"expires_at":    reflect.String,
}

// RelationsCache
var RelationsSchema = map[string]reflect.Kind{
	"id":           reflect.String,
	"primary_id":   reflect.String,
	"secondary_id": reflect.String,
	"state":        reflect.Int,
	"created_at":   reflect.String,
}

// PostsCache
var PostsSchema = map[string]reflect.Kind{
	"id":         reflect.String,
	"user_id":    reflect.String,
	"content":    reflect.String,
	"media_ids":  reflect.Slice, // UUID[]
	"visibility": reflect.Int,
	"location":   reflect.String,
	"created_at": reflect.String,
	"updated_at": reflect.String,
}

// CommentsCache
var CommentsSchema = map[string]reflect.Kind{
	"id":         reflect.String,
	"post_id":    reflect.String,
	"user_id":    reflect.String,
	"content":    reflect.String,
	"visibility": reflect.Bool,
	"created_at": reflect.String,
}

// LikesCache
var LikesSchema = map[string]reflect.Kind{
	"id":          reflect.String,
	"target_type": reflect.Int,
	"target_id":   reflect.String,
	"user_id":     reflect.String,
	"created_at":  reflect.String,
}

// MediaCache
var MediaSchema = map[string]reflect.Kind{
	"id":           reflect.String,
	"owner_id":     reflect.String,
	"storage_path": reflect.String,
	"visibility":   reflect.Bool,
	"created_at":   reflect.String,
}

// ConversationsMetaCache
var ConversationsMetaSchema = map[string]reflect.Kind{
	"id":              reflect.String,
	"type":            reflect.Int,
	"title":           reflect.String,
	"last_message_id": reflect.String,
	"state":           reflect.Int,
	"created_at":      reflect.String,
}

// ConversationMembersCache
var ConversationMembersSchema = map[string]reflect.Kind{
	"id":              reflect.String,
	"conversation_id": reflect.String,
	"user_id":         reflect.String,
	"role":            reflect.Int,
	"joined_at":       reflect.String,
	"unread_count":    reflect.Int,
}

// MessagesCache
var MessagesSchema = map[string]reflect.Kind{
	"id":              reflect.String,
	"conversation_id": reflect.String,
	"sender_id":       reflect.String,
	"message_type":    reflect.Int,
	"state":           reflect.Int,
	"content":         reflect.String,
	"attachments":     reflect.Map, // JSONB
	"created_at":      reflect.String,
}
