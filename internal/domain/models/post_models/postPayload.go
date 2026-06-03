package post_models

import "time"

// Structure interne correspondant exactement au schéma Postgres content.posts
type PostPayload struct {
	ID            int64     `bson:"id" json:"id"`
	UserID        int64     `bson:"user_id" json:"user_id"`
	Content       string    `bson:"content" json:"content"`
	Hashtags      []string  `bson:"hashtags" json:"hashtags"`
	Identifiers   []int64   `bson:"identifiers" json:"identifiers"`
	MediaIDs      []int64   `bson:"media_ids" json:"media_ids"`
	Visibility    int       `bson:"visibility" json:"visibility"`
	Location      string    `bson:"location" json:"location"`
	CreatedAt     time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" msgpack:"updated_at" bson:"updated_at"`
	LikeCount     int       `json:"like_count" msgpack:"like_count" bson:"like_count"`
	CommentCount  int       `json:"comment_count" msgpack:"comment_count" bson:"comment_count"`
	ViewCount     int       `json:"view_count" msgpack:"view_count" bson:"view_count"`
	HasMedia      bool      `json:"has_media" msgpack:"has_media" bson:"has_media"`
	Vector        []float32 `json:"vector" msgpack:"vector" bson:"vector"`
	VectorVersion int       `json:"vector_version" msgpack:"vector_version" bson:"vector_version"`
}
