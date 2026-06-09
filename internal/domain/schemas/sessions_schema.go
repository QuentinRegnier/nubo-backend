package schemas

import "reflect"

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
