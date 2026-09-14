package message_models

import "github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"

// MessageView est le DTO envoyé au client.
// Il embarque le payload brut du message et l'hydrate avec les données de l'expéditeur
// et le résumé des réactions (Fast Path).
type MessageView struct {
	MessagePayload // Embarquement : les champs (id, content, etc.) seront à la racine du JSON

	SenderUsername          string                 `json:"sender_username"`
	SenderAvatar            media_models.MediaView `json:"sender_avatar"`
	SenderAvatarCommunityID int64                  `json:"sender_avatar_community_id,omitempty"`

	// --- NOUVEAUX CHAMPS POUR LES RÉACTIONS (Fast Path) ---
	ReactionCounts map[string]int `json:"reaction_counts"` // ex: {"❤️": 1500, "😂": 500}
	UserReaction   string         `json:"user_reaction"`   // Emoji cliqué par le Caller, ou chaîne vide ("")
}
