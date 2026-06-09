package post_models

import "time"

// Structure interne correspondant exactement au schéma Postgres content.posts
type PostPayload struct {
	ID            int64     `bson:"id" json:"id" msgpack:"id"`
	UserID        int64     `bson:"user_id" json:"user_id" msgpack:"user_id"`
	Content       string    `bson:"content" json:"content" msgpack:"content"`
	Hashtags      []string  `bson:"hashtags" json:"hashtags" msgpack:"hashtags"`
	Identifiers   []int64   `bson:"identifiers" json:"identifiers" msgpack:"identifiers"`
	MediaIDs      []int64   `bson:"media_ids" json:"media_ids" msgpack:"media_ids"`
	Visibility    int       `bson:"visibility" json:"visibility" msgpack:"visibility"`
	PriorityLevel int       `bson:"priority_level" json:"priority_level" msgpack:"priority_level"` // ✅ 0=Normal, 1=Partenaire, 2=Admin
	Location      string    `bson:"location" json:"location" msgpack:"location"`
	LikeCount     int       `bson:"like_count" json:"like_count" msgpack:"like_count"`
	CommentCount  int       `bson:"like_count" json:"comment_count" msgpack:"comment_count"`
	ViewCount     int       `bson:"view_count" json:"view_count" msgpack:"view_count"`
	HasMedia      bool      `bson:"has_media" json:"has_media" msgpack:"has_media"`
	Vector        []float32 `bson:"vector" json:"vector" msgpack:"vector"`
	VectorVersion int       `bson:"vector_version" json:"vector_version" msgpack:"vector_version"`
	CreatedAt     time.Time `bson:"created_at" json:"created_at" msgpack:"created_at"`
	UpdatedAt     time.Time `bson:"updated_at" json:"updated_at" msgpack:"updated_at"`
}
