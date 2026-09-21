package service

import (
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/member_models"
)

func ToMemberSettingsLite(domainSettings member_models.MemberSettings) lite_models.MemberSettingsLite {
	return lite_models.MemberSettingsLite{
		IsMuted:           domainSettings.IsMuted,
		MuteExpireAt:      domainSettings.MuteExpireAt,
		Pinned:            domainSettings.Pinned,
		MediaAutoDownload: domainSettings.MediaAutoDownload,
		RestrictedUntil:   domainSettings.RestrictedUntil,
	}
}

func ToDomainMemberSettings(liteSettings lite_models.MemberSettingsLite) member_models.MemberSettings {
	return member_models.MemberSettings{
		IsMuted:           liteSettings.IsMuted,
		MuteExpireAt:      liteSettings.MuteExpireAt,
		Pinned:            liteSettings.Pinned,
		MediaAutoDownload: liteSettings.MediaAutoDownload,
		RestrictedUntil:   liteSettings.RestrictedUntil,
	}
}

func ToConversationSettingsLite(domainSettings conversation_models.ConversationSettings) lite_models.ConversationSettings {
	return lite_models.ConversationSettings{
		JoinApprovalRequired: domainSettings.JoinApprovalRequired,
		WritePermission:      domainSettings.WritePermission,
		SendMediaPermission:  domainSettings.SendMediaPermission,
		AddMemberPermission:  domainSettings.AddMemberPermission,
		HideSystemMessages:   domainSettings.HideSystemMessages,
		JoinWithLinkDuration: domainSettings.JoinWithLinkDuration,
		SendSurveyPermission: domainSettings.SendSurveyPermission,
	}
}

func ToDomainConversationSettings(liteSettings lite_models.ConversationSettings) conversation_models.ConversationSettings {
	return conversation_models.ConversationSettings{
		JoinApprovalRequired: liteSettings.JoinApprovalRequired,
		WritePermission:      liteSettings.WritePermission,
		SendMediaPermission:  liteSettings.SendMediaPermission,
		AddMemberPermission:  liteSettings.AddMemberPermission,
		HideSystemMessages:   liteSettings.HideSystemMessages,
		JoinWithLinkDuration: liteSettings.JoinWithLinkDuration,
		SendSurveyPermission: liteSettings.SendSurveyPermission,
	}
}

// NowMillis retourne le timestamp actuel en millisecondes UTC.
func NowMillis() int64 {
	return time.Now().UTC().UnixMilli()
}
