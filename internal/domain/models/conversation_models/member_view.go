package conversation_models

import "github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"

// MemberView est le DTO envoyé au client (notamment via WebSockets).
// Il embarque le payload brut du membre et l'hydrate avec les métadonnées de l'utilisateur.
type MemberView struct {
	MemberPayload                            // Embarquement : les champs (id, role, joined_at, etc.) seront à la racine du JSON
	Username          string                 `json:"username"`
	Avatar            media_models.MediaView `json:"avatar"`                        // ✅ RESTAURÉ
	AvatarCommunityID int64                  `json:"avatar_community_id,omitempty"` // ✅ NOUVEAU
	IsOnline          bool                   `json:"is_online"`                     // NOUVEAU : Statut de présence
}
