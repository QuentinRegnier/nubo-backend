package schemas

import "reflect"

// UserSettingsCache représente la structure du cache_service "user_settings"
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
