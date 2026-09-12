package conversation_service

import (
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
)

func DefaultMemberSettings(convType int) conversation_models.MemberSettings {
	switch convType {
	case 3:
		return conversation_models.MemberSettings{
			IsMuted:           true,
			MuteExpireAt:      -1,
			Pinned:            -1,
			MediaAutoDownload: true,
		}
	default:
		return conversation_models.MemberSettings{
			IsMuted:           false,
			MuteExpireAt:      0,
			Pinned:            -1,
			MediaAutoDownload: true,
		}
	}
}
