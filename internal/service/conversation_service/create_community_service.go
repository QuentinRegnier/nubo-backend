package conversation_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// CreateCommunity orchestre la création d'une communauté publique (Type 3) en respectant les droits et le DDD.
func CreateCommunity(ctx context.Context, callerID int64, input conversation_models.CreateCommunityInput) (int64, error) {
	// 1. IDENTITÉ & DROITS (Lecture O(1) depuis le Speed Cache)
	callerLite, err := cache_service.GetUserLite(ctx, callerID)
	if err != nil {
		return 0, err
	}

	// Grades: 0=Normal, 1=Certifié, 2=Collaborateur/Partenaire, 3=Modérateur, 4=Admin
	if callerLite.Grade < 2 {
		return 0, nubo_error.NewForbidden("INSUFFICIENT_GRADE", "Vous n'avez pas le grade requis pour créer une communauté publique.", nil)
	}

	// Règle spécifique Collaborateurs (Grade 2) : Max 1 communauté publique gérée
	if callerLite.Grade == 2 {
		// Exploitation du Speed Cache Inbox L1 pour compter sans surcharger Postgres
		inbox, errInbox := cache_service.GetInboxView(ctx, callerID, 1000, 0)
		if errInbox == nil {
			for _, item := range inbox {
				if item.Conversation.Type == 3 && item.Member.Role == 2 {
					return 0, nubo_error.NewForbidden("COMMUNITY_LIMIT_REACHED", "Vous gérez déjà une communauté publique. Une demande est nécessaire pour en créer d'autres.", nil)
				}
			}
		}
	}

	// 2. DÉFINITION DU PROPRIÉTAIRE (Owner)
	targetOwnerID := callerID

	// Si le Modérateur/Admin crée la communauté pour un tiers
	if input.OwnerID != 0 && input.OwnerID != callerID {
		if callerLite.Grade >= 3 {
			// Vérification stricte que le futur propriétaire existe
			if _, errTarget := cache_service.GetUserLite(ctx, input.OwnerID); errTarget != nil {
				return 0, nubo_error.NewBadRequest("INVALID_OWNER", "L'utilisateur spécifié comme propriétaire n'existe pas.", errTarget)
			}
			targetOwnerID = input.OwnerID
		} else {
			return 0, nubo_error.NewForbidden("INSUFFICIENT_PERMISSIONS", "Seuls les modérateurs et administrateurs peuvent céder la propriété d'une communauté à la création.", nil)
		}
	}

	// 3. CONSTRUCTION DES OBJETS MÉTIER
	now := time.Now().UTC()
	convID := pkg.GenerateID()

	laws := input.Laws
	if laws == nil {
		laws = []int{}
	}

	convPayload := conversation_models.ConversationPayload{
		ID:            convID,
		Type:          3,
		Title:         pkg.CleanStr(input.Title),
		Description:   "", // NOUVEAU
		AvatarID:      0,  // NOUVEAU
		LastMessageID: 0,
		State:         0,
		Laws:          laws,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// Le propriétaire hérite du Role 2
	ownerMember := conversation_models.MemberPayload{
		ID:              pkg.GenerateID(),
		ConversationID:  convID,
		UserID:          targetOwnerID,
		Role:            2,
		Settings:        DefaultMemberSettings(convPayload.Type),
		JoinedAt:        now,
		UnreadCount:     0,
		FrozenMessageID: 0,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	// Le Modérateur hérite d'un Role 0 s'il l'a créée pour quelqu'un d'autre
	var callerMember *conversation_models.MemberPayload
	if callerID != targetOwnerID {
		callerMember = &conversation_models.MemberPayload{
			ID:              pkg.GenerateID(),
			ConversationID:  convID,
			UserID:          callerID,
			Role:            0, // Membre classique
			Settings:        DefaultMemberSettings(convPayload.Type),
			JoinedAt:        now,
			UnreadCount:     0,
			FrozenMessageID: 0,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
	}

	// 4. SAUVEGARDE SYNCHRONE EN CACHE L1 (Disponibilité immédiate UI)
	_ = object_cache_service.SetConversationInObjectCache(ctx, convPayload)
	_ = object_cache_service.SetMemberInObjectCache(ctx, ownerMember)
	// RehydrateConversationItemInSpeedCache gère automatiquement ConvMeta, ConvMembers, Participants et l'Inbox
	cache_service.RehydrateConversationItemInSpeedCache(ctx, convPayload, ownerMember, 0)

	if callerMember != nil {
		_ = object_cache_service.SetMemberInObjectCache(ctx, *callerMember)
		cache_service.RehydrateConversationItemInSpeedCache(ctx, convPayload, *callerMember, 0)
	}

	// 5. DÉLÉGATION DE LA PERSISTANCE AUX WORKERS (Write-Behind vers L2/L3)
	// On utilise convID comme clé de partition pour que la conversation et ses membres arrivent sur le même shard
	_ = redis.EnqueueDB(ctx, convPayload.ID, convPayload.ID, redis.EntityConversation, redis.ActionCreate, convPayload, redis.TargetAll)
	_ = redis.EnqueueDB(ctx, ownerMember.ID, convPayload.ID, redis.EntityMembers, redis.ActionCreate, ownerMember, redis.TargetAll)

	if callerMember != nil {
		_ = redis.EnqueueDB(ctx, callerMember.ID, convPayload.ID, redis.EntityMembers, redis.ActionCreate, *callerMember, redis.TargetAll)
	}

	return convID, nil
}
