package schemas

import "reflect"

// CommentsCache (ou CommentsSchema) représente la structure physique d'un commentaire
var CommentsSchema = map[string]reflect.Kind{
	"id":         reflect.Int64,
	"post_id":    reflect.Int64,
	"user_id":    reflect.Int64,
	"content":    reflect.String,
	"visibility": reflect.Int,
	"like_count": reflect.Int, // ✅ NOUVEAU COMPTEUR
	"score":      reflect.Int, // ✅ NOUVEAU SCORE UNIFIÉ
	"created_at": reflect.Struct,
	"updated_at": reflect.Struct,
}
