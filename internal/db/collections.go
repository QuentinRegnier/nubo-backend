package db

import (
	"reflect"
)

// ---------------- Schemas ----------------

// UsersCache représente la structure du cache "users"
var UsersSchema = map[string]reflect.Kind{
	"id":                 reflect.Int, // UUID
	"username":           reflect.String,
	"email":              reflect.String,
	"email_verified":     reflect.Bool,
	"phone":              reflect.String,
	"phone_verified":     reflect.Bool,
	"password_hash":      reflect.String,
	"first_name":         reflect.String,
	"last_name":          reflect.String,
	"birthdate":          reflect.Struct, // Date au format string
	"sex":                reflect.Int,
	"bio":                reflect.String,
	"profile_picture_id": reflect.Int, // UUID
	"grade":              reflect.Int,
	"location":           reflect.String,
	"school":             reflect.String,
	"work":               reflect.String,
	"badges":             reflect.Slice, // Text[]
	"desactivated":       reflect.Bool,
	"banned":             reflect.Bool,
	"ban_reason":         reflect.String,
	"ban_expires_at":     reflect.Struct, // Timestamp au format string
	"created_at":         reflect.Struct,
	"updated_at":         reflect.Struct,
}

// UserSettingsCache représente la structure du cache "user_settings"
var UserSettingsSchema = map[string]reflect.Kind{
	"id":            reflect.Int,
	"user_id":       reflect.Int,
	"privacy":       reflect.Map, // JSONB
	"notifications": reflect.Map, // JSONB
	"language":      reflect.String,
	"theme":         reflect.Int,
}

// SessionsCache
var SessionsSchema = map[string]reflect.Kind{
	"id":            reflect.Int,
	"user_id":       reflect.Int,
	"refresh_token": reflect.String,
	"device_token":  reflect.String, // FCM token
	"device_info":   reflect.Map,    // JSONB
	"ip_history":    reflect.Slice,  // INET[]
	"created_at":    reflect.Struct,
	"expires_at":    reflect.Struct,
}

// RelationsCache
var RelationsSchema = map[string]reflect.Kind{
	"id":           reflect.Int,
	"primary_id":   reflect.Int,
	"secondary_id": reflect.Int,
	"state":        reflect.Int,
	"created_at":   reflect.Struct,
}

// PostsCache
var PostsSchema = map[string]reflect.Kind{
	"id":         reflect.Int,
	"user_id":    reflect.Int,
	"content":    reflect.String,
	"media_ids":  reflect.Slice, // UUID[]
	"visibility": reflect.Int,
	"location":   reflect.String,
	"created_at": reflect.Struct,
	"updated_at": reflect.Struct,
}

// CommentsCache
var CommentsSchema = map[string]reflect.Kind{
	"id":         reflect.Int,
	"post_id":    reflect.Int,
	"user_id":    reflect.Int,
	"content":    reflect.String,
	"visibility": reflect.Bool,
	"created_at": reflect.Struct,
}

// LikesCache
var LikesSchema = map[string]reflect.Kind{
	"id":          reflect.Int,
	"target_type": reflect.Int,
	"target_id":   reflect.Int,
	"user_id":     reflect.Int,
	"created_at":  reflect.Struct,
}

// MediaCache
var MediaSchema = map[string]reflect.Kind{
	"id":           reflect.Int,
	"owner_id":     reflect.Int,
	"storage_path": reflect.String,
	"visibility":   reflect.Bool,
	"created_at":   reflect.Struct,
}

// ConversationsCache
var ConversationsSchema = map[string]reflect.Kind{
	"id":                          reflect.Int,
	"type":                        reflect.Int,
	"title":                       reflect.String,
	"last_message_id":             reflect.Int,
	"last_read_by_all_message_id": reflect.Int,
	"state":                       reflect.Int,
	"created_at":                  reflect.Struct,
}

// MembersCache
var MembersSchema = map[string]reflect.Kind{
	"id":              reflect.Int,
	"conversation_id": reflect.Int,
	"user_id":         reflect.Int,
	"role":            reflect.Int,
	"joined_at":       reflect.Struct,
	"unread_count":    reflect.Int,
}

// MessagesCache
var MessagesSchema = map[string]reflect.Kind{
	"id":              reflect.Int,
	"conversation_id": reflect.Int,
	"sender_id":       reflect.Int,
	"message_type":    reflect.Int,
	"state":           reflect.Int,
	"content":         reflect.String,
	"attachments":     reflect.Map, // JSONB
	"created_at":      reflect.Struct,
}
