package service

import (
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
)

// ToMemberSettingsLite convertit le modèle métier en DTO d'infrastructure (Cache -> Redis)
func ToMemberSettingsLite(domainSettings conversation_models.MemberSettings) lite_models.MemberSettingsLite {
	return lite_models.MemberSettingsLite{
		IsMuted:           domainSettings.IsMuted,
		MuteExpireAt:      domainSettings.MuteExpireAt,
		Pinned:            domainSettings.Pinned,
		MediaAutoDownload: domainSettings.MediaAutoDownload,
	}
}

// ToDomainMemberSettings convertit le DTO du cache vers l'entité métier (Redis -> Cache)
func ToDomainMemberSettings(liteSettings lite_models.MemberSettingsLite) conversation_models.MemberSettings {
	return conversation_models.MemberSettings{
		IsMuted:           liteSettings.IsMuted,
		MuteExpireAt:      liteSettings.MuteExpireAt,
		Pinned:            liteSettings.Pinned,
		MediaAutoDownload: liteSettings.MediaAutoDownload,
	}
}
