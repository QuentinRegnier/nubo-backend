package lite_models

// TagPostItem représente la structure ultra-légère stockée et compressée
// dans le speed cache Redis (ZSETs ou listes) pour lier un post à un tag
// tout en conservant son contexte de création (Direct ou Indirect).
type TagPostItem struct {
	PostID     int64 `bson:"post_id" json:"post_id" msgpack:"post_id"`
	IsIndirect bool  `bson:"is_indirect" json:"is_indirect" msgpack:"is_indirect"`
}
