package conversation_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// RefuseCommunityRequest rejette une candidature en attente et supprime physiquement la ligne.
func RefuseCommunityRequest(ctx context.Context, callerID int64, input conversation_models.RefuseCommunityRequestInput) error {
	// 1. SÉCURITÉ : L'appelant doit être Admin (1) ou Owner (2)
	callerMem, err := security_service.LeftMember(ctx, input.ConversationID, callerID)
	if err != nil {
		return nubo_error.NewForbidden("ACCESS_DENIED", "Conversation introuvable ou accès refusé.", err)
	}
	if callerMem.Role < 1 {
		return nubo_error.NewForbidden("INSUFFICIENT_PERMISSIONS", "Vous devez être administrateur pour refuser une candidature.", nil)
	}

	// 2. RÉCUPÉRATION DU MEMBRE CIBLE (Cascade L1 -> L2 -> L3)
	var targetMem conversation_models.MemberPayload
	targetMem, err = object_cache_service.GetMemberFromObjectCache(ctx, input.ConversationID, input.TargetUserID)
	if err != nil || targetMem.ID == 0 {
		targetMem, err = mongo.MongoGetMember(input.ConversationID, input.TargetUserID)
		if err != nil || targetMem.ID == 0 {
			targetMem, err = postgres.FuncGetMember(ctx, input.ConversationID, input.TargetUserID)
			if err != nil || targetMem.ID == 0 {
				return nubo_error.NewNotFound("USER_NOT_FOUND", "Candidature introuvable.", err)
			}
		}
	}

	// 3. RÈGLE MÉTIER : Vérifier l'état d'attente
	if targetMem.Role != -3 {
		return nubo_error.NewBadRequest("INVALID_STATE", "Cet utilisateur n'est pas en attente d'approbation.", nil)
	}

	// 4. NETTOYAGE L1 (Object Cache & Speed Cache)
	_ = object_cache_service.DeleteMemberFromObjectCache(ctx, input.ConversationID, input.TargetUserID)

	// Cette fonction abstraite va supprimer ConvMembers, SRem de ConvParticipants,
	// ET faire un ZRem de l'Inbox de l'utilisateur (la communauté disparaît de son écran).
	_ = cache_service.RemoveMemberFromSpeedCache(ctx, input.ConversationID, input.TargetUserID)

	// 5. ENVOI AUX WORKERS (Write-Behind)
	// On envoie un redis.ActionDelete. Le postgres_batch et le mongo_batch
	// exécuteront alors un Delete() pur et dur sur l'ID de ce membre.
	err = redis.EnqueueDB(ctx, targetMem.ID, input.ConversationID, redis.EntityMembers, redis.ActionDelete, targetMem, redis.TargetAll)

	return err
}
