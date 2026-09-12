package message_models

import "github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"

// MessageView est le DTO envoyé au client.
// Il embarque le payload brut du message et l'hydrate avec les données de l'expéditeur.
type MessageView struct {
	MessagePayload // Embarquement : les champs (id, content, etc.) seront à la racine du JSON

	SenderUsername          string                 `json:"sender_username"`
	SenderAvatar            media_models.MediaView `json:"sender_avatar"`                        // ✅ RESTAURÉ
	SenderAvatarCommunityID int64                  `json:"sender_avatar_community_id,omitempty"` // ✅ NOUVEAU : Rempli uniquement en mode Communauté
}
