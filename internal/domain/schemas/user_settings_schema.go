package schemas

import "reflect"

// UserSettingsSchema représente la structure du cache_service "user_settings"
var UserSettingsSchema = map[string]reflect.Kind{
	"id":                  reflect.Int64,
	"user_id":             reflect.Int64,
	"privacy":             reflect.Map, // JSONB
	"notifications":       reflect.Map, // JSONB
	"display_and_content": reflect.Map, // NOUVEAU : JSONB (Remplace "language" et "theme")
	"telemetry_vector":    reflect.Slice,
	"telemetry_tags":      reflect.Slice,
	"telemetry_timestamp": reflect.Int64,
	"created_at":          reflect.Struct,
	"updated_at":          reflect.Struct,
}
