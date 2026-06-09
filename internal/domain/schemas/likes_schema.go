package schemas

import "reflect"

// LikesCache
var LikesSchema = map[string]reflect.Kind{
	"id":          reflect.Int64,
	"target_type": reflect.Int,
	"target_id":   reflect.Int64,
	"user_id":     reflect.Int64,
	"created_at":  reflect.Struct,
}
