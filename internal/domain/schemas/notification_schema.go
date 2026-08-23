package schemas

import "reflect"

// NotificationsSchema représente la structure physique d'une notification
var NotificationsSchema = map[string]reflect.Kind{
	"id":         reflect.Int64,
	"user_id":    reflect.Int64,
	"actor_id":   reflect.Int64,
	"type":       reflect.String,
	"target_id":  reflect.Int64,
	"is_read":    reflect.Bool,
	"created_at": reflect.Struct,
}
