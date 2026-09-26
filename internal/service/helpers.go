package service

import (
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/member_models"
)

// ############################################################################
// # MAPPERS ET UTILITAIRES DE MODÈLES (DOMAIN <-> LITE)
// ############################################################################

// ToMemberSettingsLite compresse les paramètres d'un membre pour le Speed Cache RAM (MsgPack).
func ToMemberSettingsLite(domainSettings member_models.MemberSettings) lite_models.MemberSettingsLite {
	return lite_models.MemberSettingsLite{
		IsMuted:           domainSettings.IsMuted,
		MuteExpireAt:      domainSettings.MuteExpireAt,
		Pinned:            domainSettings.Pinned,
		MediaAutoDownload: domainSettings.MediaAutoDownload,
		RestrictedUntil:   domainSettings.RestrictedUntil,
	}
}

// ToDomainMemberSettings décompresse les paramètres d'un membre pour la logique métier.
func ToDomainMemberSettings(liteSettings lite_models.MemberSettingsLite) member_models.MemberSettings {
	return member_models.MemberSettings{
		IsMuted:           liteSettings.IsMuted,
		MuteExpireAt:      liteSettings.MuteExpireAt,
		Pinned:            liteSettings.Pinned,
		MediaAutoDownload: liteSettings.MediaAutoDownload,
		RestrictedUntil:   liteSettings.RestrictedUntil,
	}
}

// ToConversationSettingsLite compresse les paramètres d'une conversation pour le Speed Cache.
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

// ToDomainConversationSettings décompresse les paramètres d'une conversation pour la logique métier.
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
