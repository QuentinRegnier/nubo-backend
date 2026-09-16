package service

import (
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
)

func ToMemberSettingsLite(domainSettings conversation_models.MemberSettings) lite_models.MemberSettingsLite {
	return lite_models.MemberSettingsLite{
		IsMuted:           domainSettings.IsMuted,
		MuteExpireAt:      domainSettings.MuteExpireAt,
		Pinned:            domainSettings.Pinned,
		MediaAutoDownload: domainSettings.MediaAutoDownload,
	}
}

func ToDomainMemberSettings(liteSettings lite_models.MemberSettingsLite) conversation_models.MemberSettings {
	return conversation_models.MemberSettings{
		IsMuted:           liteSettings.IsMuted,
		MuteExpireAt:      liteSettings.MuteExpireAt,
		Pinned:            liteSettings.Pinned,
		MediaAutoDownload: liteSettings.MediaAutoDownload,
	}
}

func ToConversationSettingsLite(domainSettings conversation_models.ConversationSettings) lite_models.ConversationSettingsLite {
	return lite_models.ConversationSettingsLite{
		JoinApprovalRequired: domainSettings.JoinApprovalRequired,
		WritePermission:      domainSettings.WritePermission,
		SendMediaPermission:  domainSettings.SendMediaPermission,
		AddMemberPermission:  domainSettings.AddMemberPermission,
		HideSystemMessages:   domainSettings.HideSystemMessages,
	}
}

func ToDomainConversationSettings(liteSettings lite_models.ConversationSettingsLite) conversation_models.ConversationSettings {
	return conversation_models.ConversationSettings{
		JoinApprovalRequired: liteSettings.JoinApprovalRequired,
		WritePermission:      liteSettings.WritePermission,
		SendMediaPermission:  liteSettings.SendMediaPermission,
		AddMemberPermission:  liteSettings.AddMemberPermission,
		HideSystemMessages:   liteSettings.HideSystemMessages,
	}
}

// NowMillis retourne le timestamp actuel en millisecondes UTC.
func NowMillis() int64 {
	return time.Now().UTC().UnixMilli()
}
