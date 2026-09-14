package conversation_service

import (
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
)

func DefaultMemberSettings(convType int) conversation_models.MemberSettings {
	switch convType {
	case 3:
		return conversation_models.MemberSettings{
			IsMuted:           2,
			MuteExpireAt:      -1,
			Pinned:            -1,
			MediaAutoDownload: true,
		}
	default:
		return conversation_models.MemberSettings{
			IsMuted:           0,
			MuteExpireAt:      0,
			Pinned:            -1,
			MediaAutoDownload: true,
		}
	}
}

func DefaultConversationSettings(convType int) conversation_models.ConversationSettings {
	switch convType {
	case 3: // Communautés (Vitrine par défaut)
		return conversation_models.ConversationSettings{
			JoinApprovalRequired: false,
			WritePermission:      2,    // Annonces & Threads
			SendMediaPermission:  1,    // Bloque les médias aux membres
			AddMemberPermission:  0,    // Bloque l'ajout aux membres
			HideSystemMessages:   true, // On cache le bruit de fond
		}
	default: // MP (Type 0) ou inconnu
		return conversation_models.ConversationSettings{
			JoinApprovalRequired: false,
			WritePermission:      0,
			SendMediaPermission:  0,
			AddMemberPermission:  0,
			HideSystemMessages:   false,
		}
	}
}
