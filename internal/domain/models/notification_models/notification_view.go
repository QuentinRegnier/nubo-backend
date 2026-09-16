package notification_models

import (
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
)

// NotificationView est le DTO envoyé au client avec les données pré-hydratées
type NotificationView struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	TargetID  int64  `json:"target_id"` // ID du Post, Commentaire, etc.
	IsRead    bool   `json:"is_read"`
	CreatedAt int64  `json:"created_at"`

	// Données de l'acteur hydratées en O(1)
	ActorID                int64                  `json:"actor_id"`
	ActorUsername          string                 `json:"actor_username"`
	ActorAvatar            media_models.MediaView `json:"actor_avatar"`                        // ✅ RESTAURÉ : Utilisé pour 90% du trafic (HTTP et MP)
	ActorAvatarCommunityID int64                  `json:"actor_avatar_community_id,omitempty"` // ✅ NOUVEAU : Fallback pour le mode Twitch (0 par défaut)
}
