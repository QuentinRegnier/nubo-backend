package domain

import (
	"reflect"
)

// ---------------- Schemas ----------------

// UsersCache représente la structure du cache "users"
var UsersSchema = map[string]reflect.Kind{
	"id":                 reflect.Int64, // UUID
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
	"profile_picture_id": reflect.Int64, // UUID
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
	"id":            reflect.Int64,
	"user_id":       reflect.Int64,
	"privacy":       reflect.Map, // JSONB
	"notifications": reflect.Map, // JSONB
	"language":      reflect.String,
	"theme":         reflect.Int,
	"created_at":    reflect.Struct,
	"updated_at":    reflect.Struct,
}

// SessionsCache
var SessionsSchema = map[string]reflect.Kind{
	"id":             reflect.Int64,
	"user_id":        reflect.Int64,
	"master_token":   reflect.String,
	"device_token":   reflect.String, // FCM token
	"device_info":    reflect.Map,    // JSONB
	"ip_history":     reflect.Slice,  // INET[]
	"current_secret": reflect.String, // Ratchet : Secret N+1
	"last_secret":    reflect.String, // Ratchet : Secret N
	"last_jwt":       reflect.String, // Dernier JWT émis
	"tolerance_time": reflect.Struct, // Timestamp limite (time.Time)
	"created_at":     reflect.Struct,
	"expires_at":     reflect.Struct,
}

// RelationsCache
var RelationsSchema = map[string]reflect.Kind{
	"id":           reflect.Int64,
	"primary_id":   reflect.Int64,
	"secondary_id": reflect.Int64,
	"state":        reflect.Int,
	"created_at":   reflect.Struct,
	"updated_at":   reflect.Struct,
}

// PostsCache
var PostsSchema = map[string]reflect.Kind{
	"id":          reflect.Int64,
	"user_id":     reflect.Int64,
	"content":     reflect.String,
	"hashtags":    reflect.Slice, // Text[]
	"identifiers": reflect.Slice, // UUID[]
	"media_ids":   reflect.Slice, // UUID[]
	"visibility":  reflect.Int,
	"location":    reflect.String,
	"created_at":  reflect.Struct,
	"updated_at":  reflect.Struct,
}

// CommentsCache
var CommentsSchema = map[string]reflect.Kind{
	"id":         reflect.Int64,
	"post_id":    reflect.Int64,
	"user_id":    reflect.Int64,
	"content":    reflect.String,
	"visibility": reflect.Bool,
	"created_at": reflect.Struct,
	"updated_at": reflect.Struct,
}

// LikesCache
var LikesSchema = map[string]reflect.Kind{
	"id":          reflect.Int64,
	"target_type": reflect.Int,
	"target_id":   reflect.Int64,
	"user_id":     reflect.Int64,
	"created_at":  reflect.Struct,
}

// MediaCache
var MediaSchema = map[string]reflect.Kind{
	"id":           reflect.Int64,
	"owner_id":     reflect.Int64,
	"storage_path": reflect.String,
	"visibility":   reflect.Bool,
	"created_at":   reflect.Struct,
	"updated_at":   reflect.Struct,
}

// ConversationsCache
var ConversationsSchema = map[string]reflect.Kind{
	"id":                          reflect.Int64,
	"type":                        reflect.Int,
	"title":                       reflect.String,
	"last_message_id":             reflect.Int64,
	"last_read_by_all_message_id": reflect.Int64,
	"state":                       reflect.Int,
	"created_at":                  reflect.Struct,
	"updated_at":                  reflect.Struct,
}

// MembersCache
var MembersSchema = map[string]reflect.Kind{
	"id":              reflect.Int64,
	"conversation_id": reflect.Int64,
	"user_id":         reflect.Int64,
	"role":            reflect.Int,
	"joined_at":       reflect.Struct,
	"unread_count":    reflect.Int,
	"created_at":      reflect.Struct,
	"updated_at":      reflect.Struct,
}

// MessagesCache
var MessagesSchema = map[string]reflect.Kind{
	"id":              reflect.Int64,
	"conversation_id": reflect.Int64,
	"sender_id":       reflect.Int64,
	"message_type":    reflect.Int,
	"state":           reflect.Int,
	"content":         reflect.String,
	"attachments":     reflect.Map, // JSONB
	"created_at":      reflect.Struct,
	"updated_at":      reflect.Struct,
}
