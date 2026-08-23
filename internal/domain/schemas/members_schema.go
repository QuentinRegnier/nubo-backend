package schemas

import "reflect"

var MembersSchema = map[string]reflect.Kind{
	"id":                reflect.Int64,
	"conversation_id":   reflect.Int64,
	"user_id":           reflect.Int64,
	"role":              reflect.Int,
	"joined_at":         reflect.Struct,
	"unread_count":      reflect.Int,
	"frozen_message_id": reflect.Int64, // ✅ NOUVEAU
	"created_at":        reflect.Struct,
	"updated_at":        reflect.Struct,
}
