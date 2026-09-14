package schemas

import "reflect"

// MessageReactionsSchema représente la structure physique d'une réaction à un message.
var MessageReactionsSchema = map[string]reflect.Kind{
	"id":         reflect.Int64,
	"message_id": reflect.Int64,
	"user_id":    reflect.Int64,
	"reaction":   reflect.String,
	"created_at": reflect.Struct,
}
