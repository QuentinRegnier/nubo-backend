package schemas

import "reflect"

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
