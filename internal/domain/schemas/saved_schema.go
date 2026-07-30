package schemas

import "reflect"

// SavedCache représente la structure de la collection Mongo "saved"
var SavedSchema = map[string]reflect.Kind{
	"id":         reflect.Int64,
	"user_id":    reflect.Int64,
	"post_id":    reflect.Int64,
	"created_at": reflect.Struct,
}
