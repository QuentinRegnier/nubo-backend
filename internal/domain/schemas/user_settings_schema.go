package schemas

import "reflect"

// UserSettingsCache représente la structure du cache_service "user_settings"
var UserSettingsSchema = map[string]reflect.Kind{
	"id":                  reflect.Int64,
	"user_id":             reflect.Int64,
	"privacy":             reflect.Map, // JSONB
	"notifications":       reflect.Map, // JSONB
	"language":            reflect.String,
	"theme":               reflect.Int,
	"telemetry_vector":    reflect.Slice, // NOUVEAU : REAL[] -> []float32
	"telemetry_tags":      reflect.Slice, // NOUVEAU : TEXT[] -> []string
	"telemetry_timestamp": reflect.Int64, // NOUVEAU : BIGINT -> int64
	"created_at":          reflect.Struct,
	"updated_at":          reflect.Struct,
}
