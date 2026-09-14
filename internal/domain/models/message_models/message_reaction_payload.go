package message_models

import "time"

// MessageReactionPayload correspond au schéma Postgres messaging.message_reactions
type MessageReactionPayload struct {
	ID        int64     `json:"id" bson:"id"`
	MessageID int64     `json:"message_id" bson:"message_id"`
	UserID    int64     `json:"user_id" bson:"user_id"`
	Reaction  string    `json:"reaction" bson:"reaction"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
}
