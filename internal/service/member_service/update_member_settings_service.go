package member_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/member_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// UpdateMemberSettings gère la mise à jour partielle des paramètres d'un membre pour une conversation spécifique.
func UpdateMemberSettings(ctx context.Context, callerID int64, input member_models.UpdateMemberSettingsInput) (member_models.UpdateMemberSettingsOutput, error) {
	// 1. SÉCURITÉ : Récupération sécurisée du membre (Cascade L1->L2->L3 avec vérification d'appartenance)
	// LeftMember renvoie une nubo_error 403 propre si l'utilisateur n'est pas dans la conversation ou est banni.
	mem, err := security_service.LeftMember(ctx, input.ConversationID, callerID)
	if err != nil {
		return member_models.UpdateMemberSettingsOutput{}, err
	}

	// 2. MODIFICATION PARTIELLE DU DICTIONNAIRE (Seulement si la valeur a été fournie)
	mem.Settings.IsMuted = input.IsMuted
	mem.Settings.MuteExpireAt = input.MuteExpiresAt
	mem.Settings.MediaAutoDownload = input.MediaAutoDownload
	mem.UpdatedAt = domain.NowMillis()

	// 3. MISE À JOUR SYNCHRONE DU CACHE L1 (Object Cache LFU)
	_ = object_cache_service.SetMemberInObjectCache(ctx, mem)

	// 4. MISE À JOUR SYNCHRONE DU SPEED CACHE (Lite Models pour la barre de recherche/inbox)
	_ = cache_service.UpdateMemberSpeedCache(ctx, lite_models.MemberLiteRequest{
		ConversationID:  mem.ConversationID,
		UserID:          mem.UserID,
		Role:            mem.Role,
		Settings:        service.ToMemberSettingsLite(mem.Settings),
		UnreadCount:     mem.UnreadCount,
		FrozenMessageID: mem.FrozenMessageID,
		JoinedAt:        mem.JoinedAt,
	})

	// 5. DÉLÉGATION À LA FILE ASYNCHRONE (Write-Behind)
	// PartitionKey = input.ConversationID pour s'assurer que les événements de cette conversation soient traités dans le bon ordre

	output := member_models.UpdateMemberSettingsOutput{}

	// ========================================================================
	// MARQUAGE DU TEMPS (DIRTY FLAG)
	// ========================================================================
	// Placé TOUT À LA FIN de la fonction. Cela écrase tout timestamp qui aurait
	// pu être généré précédemment (par ex. à l'intérieur de AddMembersToConversation)
	// et garantit que le client reçoit la date de la fin absolue de la transaction.
	timestampMs := cache_service.TouchInboxActivity(ctx, callerID)
	output.InboxUpdateAt = domain.TimeToMillis(time.UnixMilli(timestampMs))

	return output, redis.EnqueueDB(ctx, mem.ID, input.ConversationID, redis.EntityMembers, redis.ActionUpdate, mem, redis.TargetAll)
}
