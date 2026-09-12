package conversation_models

import (
	"time"
)

// ConversationPayload correspond exactement au schéma Postgres messaging.conversations
type ConversationPayload struct {
	ID            int64     `json:"id" bson:"id"`
	Type          int       `json:"type" bson:"type"` // -2, -1, 0, 1, 2, 3
	Title         string    `json:"title" bson:"title"`
	Description   string    `json:"description" bson:"description"`
	AvatarID      int64     `json:"avatar_id" bson:"avatar_id"`
	LastMessageID int64     `json:"last_message_id" bson:"last_message_id"`
	State         int       `json:"state" bson:"state"` // 0=active, 1=archived
	Laws          []int     `json:"laws" bson:"laws"`   // Tableau de règles
	CreatedAt     time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" bson:"updated_at"`
}
