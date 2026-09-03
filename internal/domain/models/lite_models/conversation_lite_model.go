package lite_models

type ConvLiteRequest struct {
	ID            int64  `bson:"id" json:"id"`
	Type          int    `bson:"type" json:"type"`
	Title         string `bson:"title" json:"title"`
	LastMessageID int64  `bson:"last_message_id" json:"last_message_id"`
}
