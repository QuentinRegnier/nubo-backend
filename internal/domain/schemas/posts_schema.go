package schemas

import "reflect"

// PostsCache
var PostsSchema = map[string]reflect.Kind{
	"id":             reflect.Int64,
	"user_id":        reflect.Int64,
	"content":        reflect.String,
	"hashtags":       reflect.Slice, // Text[]
	"identifiers":    reflect.Slice, // Bigint[]
	"media_ids":      reflect.Slice, // Bigint[]
	"visibility":     reflect.Int,
	"priority_level": reflect.Int,
	"location":       reflect.String,
	"like_count":     reflect.Int,
	"comment_count":  reflect.Int,
	"view_count":     reflect.Int,
	"has_media":      reflect.Bool,
	"vector":         reflect.Slice, // REAL[] -> []float32
	"vector_version": reflect.Int,   // Integer
	"created_at":     reflect.Struct,
	"updated_at":     reflect.Struct,
}
