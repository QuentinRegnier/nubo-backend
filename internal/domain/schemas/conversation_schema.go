package schemas

import "reflect"

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
