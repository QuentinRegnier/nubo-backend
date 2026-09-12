package conversation_service

import (
	"context"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// SuggestMembers orchestre la suggestion de membres en réutilisant le Speed Cache relationnel.
func SuggestMembers(ctx context.Context, callerID int64, input conversation_models.SuggestMemberInput) (conversation_models.SuggestMemberOutput, error) {
	// 1. SÉCURITÉ : Le caller doit être membre actif du groupe (Role >= 0)
	callerMem, err := security_service.LeftMember(ctx, input.ConversationID, callerID)
	if err != nil || callerMem.Role < 0 {
		return conversation_models.SuggestMemberOutput{}, nubo_error.NewForbidden("NOT_A_MEMBER", "Accès refusé : vous ne faites pas partie de cette conversation.", err)
	}

	limit := input.Limit
	if limit == 0 {
		limit = 20
	}

	// 2. RÉCUPÉRATION DES MEMBRES ACTUELS (Pour les exclure des suggestions)
	participantsStr, _ := redis.ConvParticipants.SMembers(ctx, input.ConversationID)
	existingMembers := make(map[int64]bool, len(participantsStr))
	for _, pStr := range participantsStr {
		if id, errParse := strconv.ParseInt(pStr, 10, 64); errParse == nil {
			existingMembers[id] = true
		}
	}

	var validUsers []auth_models.UserLiteView
	currentOffset := input.Offset
	maxIterations := 3 // Sécurité anti-boucle infinie

	// 3. BOUCLE DE COMPENSATION (Remplit le tableau en gérant les "trous" créés par le filtrage)
	for int64(len(validUsers)) < limit && maxIterations > 0 {
		fetchLimit := limit * 2 // On tire plus large en RAM pour anticiper les rejets
		var candidates []lite_models.UserLiteRequest

		// AIGUILLAGE DDD
		if input.Query == "" {
			// A. Recherche vide : On tire le ZSET des abonnements/amis existant (Ami > Abonné)
			candidates, _ = cache_service.GetAddableUsersFromSpeedCache(ctx, callerID, fetchLimit, currentOffset, false)
		} else {
			// B. Autocomplétion : ZSET Lexicographique Global
			candidates, _ = cache_service.SearchUserByPrefix(ctx, input.Query, fetchLimit)
		}

		if len(candidates) == 0 {
			break // Plus de candidats disponibles en RAM
		}

		// 4. FILTRAGE ET MATRICE DE CONFIDENTIALITÉ
		for _, target := range candidates {
			if int64(len(validUsers)) >= limit {
				break
			}

			// Exclusion de soi-même ou d'un membre déjà présent
			if target.ID == callerID || existingMembers[target.ID] {
				continue
			}

			// Lecture O(1) du lien social
			relationState := cache_service.RelationValue(ctx, callerID, target.ID)

			// RÈGLE DE BAN : Rejet direct
			if relationState == -1 {
				continue
			}

			// VERROU : Droit de communication absolu (Matrice copiée de AddMembers)
			canCommunicate := false
			switch target.ConversationPermission {
			case 0: // Public
				canCommunicate = true
			case 1: // Abonnés
				canCommunicate = relationState >= 1
			case 2: // Amis
				canCommunicate = relationState == 2
			}

			if !canCommunicate {
				continue
			}

			// 5. HYDRATATION DE L'AVATAR ET DE LA PRÉSENCE
			var avatar media_models.MediaView
			if target.ProfilePictureID > 0 {
				if view, errMedia := media_service.GenerateMediaViewCascade(ctx, target.ProfilePictureID, target.ID, 0, callerID); errMedia == nil {
					avatar = view
				}
			}

			validUsers = append(validUsers, auth_models.UserLiteView{
				User:     target,
				Avatar:   avatar,
				IsOnline: cache_service.IsUserOnline(ctx, target.ID), // NOUVEAU (O(1))
			})
		}

		// La recherche lexicographique ne supportant pas l'offset de base, on ne boucle pas
		if input.Query != "" {
			break
		}

		currentOffset += fetchLimit
		maxIterations--
	}

	// Prévention du retour 'null' en JSON
	if validUsers == nil {
		validUsers = make([]auth_models.UserLiteView, 0)
	}

	return conversation_models.SuggestMemberOutput{Users: validUsers}, nil
}
