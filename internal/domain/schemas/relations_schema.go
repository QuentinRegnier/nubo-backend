package schemas

import "reflect"

// RelationsCache
var RelationsSchema = map[string]reflect.Kind{
	"id":           reflect.Int64,
	"primary_id":   reflect.Int64,
	"secondary_id": reflect.Int64,
	"state":        reflect.Int,
	"created_at":   reflect.Struct,
	"updated_at":   reflect.Struct,
}
