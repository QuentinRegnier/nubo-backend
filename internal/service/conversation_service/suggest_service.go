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
	"github.com/vmihailenco/msgpack/v5"
)

// SuggestContacts orchestre la suggestion de membres pour les groupes et les MP.
func SuggestContacts(ctx context.Context, callerID int64, input conversation_models.SuggestInput) (conversation_models.SuggestOutput, error) {
	excludedIDs := make(map[int64]bool)
	isGroupContext := input.ConversationID > 0

	// 1. GESTION DU CONTEXTE ET DES EXCLUSIONS
	if isGroupContext {
		// A. Contexte Groupe : Le caller doit être membre et on exclut les membres actuels
		callerMem, err := security_service.LeftMember(ctx, input.ConversationID, callerID)
		if err != nil || callerMem.Role < 0 {
			return conversation_models.SuggestOutput{}, nubo_error.NewForbidden("NOT_A_MEMBER", "Accès refusé : vous ne faites pas partie de ce groupe.", err)
		}

		participantsStr, _ := redis.ConvParticipants.SMembers(ctx, input.ConversationID)
		for _, pStr := range participantsStr {
			if id, errParse := strconv.ParseInt(pStr, 10, 64); errParse == nil {
				excludedIDs[id] = true
			}
		}
	} else {
		// B. Contexte MP : On exclut les utilisateurs avec qui on a déjà une conversation active
		// On lit l'inbox du Speed Cache (ultra-rapide car cappée à 100[cite: 9])
		convIDStrings, _ := redis.UserInbox.ZRevRange(ctx, callerID, 0, -1)
		var convIDs []int64
		for _, idStr := range convIDStrings {
			if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
				convIDs = append(convIDs, id)
			}
		}

		metaRes, _ := redis.ConvMeta.GetMany(ctx, convIDs)
		for cid, data := range metaRes.Found {
			var meta lite_models.ConvLiteRequest
			if err := msgpack.Unmarshal(data, &meta); err == nil && meta.Type == 0 {
				// C'est un MP, on cherche l'autre participant
				participants, _ := redis.ConvParticipants.SMembers(ctx, cid)
				for _, pStr := range participants {
					if id, errParse := strconv.ParseInt(pStr, 10, 64); errParse == nil && id != callerID {
						excludedIDs[id] = true
					}
				}
			}
		}
	}

	// 2. RÉCUPÉRATION DES CANDIDATS (Aiguillage Query vs Carnet d'adresses)
	var validUsers []auth_models.UserLiteView
	currentOffset := input.Offset
	maxIterations := 3 // Sécurité anti-boucle infinie

	for int64(len(validUsers)) < input.Limit && maxIterations > 0 {
		fetchLimit := input.Limit * 2
		var candidates []lite_models.UserLiteRequest

		if input.Query == "" {
			candidates, _ = cache_service.GetAddableUsersFromSpeedCache(ctx, callerID, fetchLimit, currentOffset, false)
		} else {
			candidates, _ = cache_service.SearchUserByPrefix(ctx, input.Query, fetchLimit)
			if len(candidates) == 0 {
				break
			}
		}

		// 3. FILTRAGE ET MATRICE DE CONFIDENTIALITÉ
		for _, target := range candidates {
			if int64(len(validUsers)) >= input.Limit {
				break
			}

			// Exclusion de soi-même ou d'un utilisateur déjà présent/existant
			if target.ID == callerID || excludedIDs[target.ID] {
				continue
			}

			relationState := cache_service.RelationValue(ctx, callerID, target.ID)
			if relationState == -1 {
				continue // Utilisateur bloqué
			}

			canCommunicate := false

			// AIGUILLAGE DU VERROU DE CONFIDENTIALITÉ
			if isGroupContext {
				// On évalue la permission d'ajout aux groupes
				switch target.AddGroupPermission {
				case 0:
					canCommunicate = true
				case 1:
					canCommunicate = (relationState == 2)
				case 2:
					canCommunicate = false // Invitation uniquement
				}
			} else {
				// On évalue la permission de Message Privé
				switch target.ConversationPermission {
				case 0:
					canCommunicate = true
				case 1:
					canCommunicate = (relationState >= 1)
				case 2:
					canCommunicate = (relationState == 2)
				}
			}

			if !canCommunicate {
				continue
			}

			// 4. HYDRATATION DE L'AVATAR
			var avatar media_models.MediaView
			if target.ProfilePictureID > 0 {
				if view, errMedia := media_service.GenerateMediaViewCascade(ctx, target.ProfilePictureID, target.ID, 0, callerID); errMedia == nil {
					avatar = view
				}
			}

			validUsers = append(validUsers, auth_models.UserLiteView{
				User:     target,
				Avatar:   avatar,
				IsOnline: cache_service.IsUserOnline(ctx, target.ID),
			})
		}

		if input.Query != "" {
			break // Pas d'offset géré sur l'index lexicographique
		}
		currentOffset += fetchLimit
		maxIterations--
	}

	if validUsers == nil {
		validUsers = make([]auth_models.UserLiteView, 0)
	}

	return conversation_models.SuggestOutput{Users: validUsers}, nil
}
