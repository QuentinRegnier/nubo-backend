package message_models

import "time"

// MessagePayload correspond exactement au schéma Postgres messaging.messages
type MessagePayload struct {
	ID             int64          `json:"id" bson:"id"`
	ConversationID int64          `json:"conversation_id" bson:"conversation_id"`
	SenderID       int64          `json:"sender_id" bson:"sender_id"`
	MessageType    int            `json:"message_type" bson:"message_type"`
	Visibility     bool           `json:"visibility" bson:"visibility"`
	Content        string         `json:"content" bson:"content"`
	Attachments    map[string]any `json:"attachments" bson:"attachments"` // JSONB polymorphe
	CreatedAt      time.Time      `json:"created_at" bson:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at" bson:"updated_at"`
}
