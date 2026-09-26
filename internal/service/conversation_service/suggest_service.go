package conversation_service

import (
	"context"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
	"github.com/vmihailenco/msgpack/v5"
)

// ############################################################################
// # SERVICE : SUGGESTION DE CONTACTS (AUTCOMPLÉTION & LISTES)
// ############################################################################

// SuggestContacts orchestre la suggestion de membres selon l'intention demandée (Groupe, DM, Tag, Mention).
// Elle filtre automatiquement les utilisateurs qui ne peuvent pas être contactés.
func SuggestContacts(ctx context.Context, callerID int64, input conversation_models.SuggestInput) (conversation_models.SuggestOutput, error) {
	excludedUserIDsMap := make(map[int64]bool)
	isContextGroup := input.ConversationID > 0

	// Déduction de l'intention métier (Fallback)
	intentAction := input.Intent
	if intentAction == "" {
		if isContextGroup {
			intentAction = "group"
		} else {
			intentAction = "dm"
		}
	}

	// ── ÉTAPE 1 : GESTION DU CONTEXTE ET DES EXCLUSIONS PRÉALABLES ──────────

	if isContextGroup && intentAction == "group" {
		callerMemberPayload, errSecurity := security_service.LeftMember(ctx, input.ConversationID, callerID)
		if errSecurity != nil || callerMemberPayload.Role < variables.MemberRoleNormal {
			return conversation_models.SuggestOutput{}, nubo_error.NewForbidden(nubo_error.CodeForbidden, "Accès refusé : vous ne faites pas partie de ce groupe.", errSecurity)
		}

		// On exclut les utilisateurs déjà présents dans le groupe
		participantsStringList, _ := redis.ConvParticipants.SMembers(ctx, input.ConversationID)
		for _, participantStr := range participantsStringList {
			if parsedID, errParse := strconv.ParseInt(participantStr, 10, 64); errParse == nil {
				excludedUserIDsMap[parsedID] = true
			}
		}
	} else if intentAction == "dm" {
		// On exclut les utilisateurs avec lesquels on a déjà un MP actif
		inboxConversationIDStrings, _ := redis.UserInbox.ZRevRange(ctx, callerID, 0, -1)
		var inboxConversationIDs []int64
		for _, idStr := range inboxConversationIDStrings {
			if id, errParse := strconv.ParseInt(idStr, 10, 64); errParse == nil {
				inboxConversationIDs = append(inboxConversationIDs, id)
			}
		}

		conversationMetasBatch, _ := redis.ConvMeta.GetMany(ctx, inboxConversationIDs)
		for convID, rawData := range conversationMetasBatch.Found {
			var liteConversation lite_models.ConvLiteRequest
			if errUnpack := msgpack.Unmarshal(rawData, &liteConversation); errUnpack == nil && liteConversation.Type == variables.ConversationTypeDirect {

				// Pour chaque MP, on exclut l'autre participant
				directParticipantsStringList, _ := redis.ConvParticipants.SMembers(ctx, convID)
				for _, pStr := range directParticipantsStringList {
					if targetID, errParse := strconv.ParseInt(pStr, 10, 64); errParse == nil && targetID != callerID {
						excludedUserIDsMap[targetID] = true
					}
				}
			}
		}
	}

	// ── ÉTAPE 2 : RÉCUPÉRATION ET FILTRAGE DES CANDIDATS ────────────────────

	var validatedUsersList []auth_models.UserLiteView
	currentOffset := input.Offset
	maxFetchIterations := 3 // Prévention de boucle infinie sur une DB immense très restrictive

	for int64(len(validatedUsersList)) < input.Limit && maxFetchIterations > 0 {
		fetchBatchLimit := input.Limit * 2
		var potentialCandidates []lite_models.UserLiteRequest

		// Route A : Suggestion de base (Amis > Abonnés) vs Route B : Recherche textuelle
		if input.Query == "" {
			var errCache error
			potentialCandidates, errCache = cache_service.GetAddableUsersFromSpeedCache(ctx, callerID, fetchBatchLimit, currentOffset, false)
			if errCache != nil {
				logger.Log.Warn().Err(errCache).Msg("Erreur lors de la récupération des AddableUsers en RAM")
			}
		} else {
			var errCache error
			potentialCandidates, errCache = cache_service.SearchUserByPrefix(ctx, input.Query, fetchBatchLimit)
			if errCache != nil {
				logger.Log.Warn().Err(errCache).Msg("Erreur lors de la recherche par préfixe en RAM")
			}
			if len(potentialCandidates) == 0 {
				break
			}
		}

		// ── ÉTAPE 3 : APPLICATION DE LA MATRICE DE CONFIDENTIALITÉ ──────────

		for _, targetUserLite := range potentialCandidates {
			if int64(len(validatedUsersList)) >= input.Limit {
				break
			}

			if targetUserLite.ID == callerID || excludedUserIDsMap[targetUserLite.ID] {
				continue
			}

			// (0=Rien, 1=Abonné, 2=Ami, -1=Bloqué)
			relationState := cache_service.RelationValue(ctx, callerID, targetUserLite.ID)
			if relationState == variables.RelationStateBlocked {
				continue // Exclusion totale (Blocage)
			}

			isCommunicationAllowed := false

			// AIGUILLAGE SÉCURITAIRE DYNAMIQUE
			switch intentAction {
			case "group":
				switch targetUserLite.AddGroupPermission {
				case 0:
					isCommunicationAllowed = true
				case 1:
					isCommunicationAllowed = (relationState == 2)
				case 2:
					isCommunicationAllowed = false // Invitation uniquement
				}
			case "dm":
				switch targetUserLite.ConversationPermission {
				case 0:
					isCommunicationAllowed = true
				case 1:
					isCommunicationAllowed = (relationState >= 1)
				case 2:
					isCommunicationAllowed = (relationState == 2)
				case 3:
					isCommunicationAllowed = false
				}
			case "tag":
				switch targetUserLite.AllowTagging {
				case 0:
					isCommunicationAllowed = true
				case 1:
					isCommunicationAllowed = (relationState >= 1)
				case 2:
					isCommunicationAllowed = (relationState == 2)
				}
			case "mention":
				switch targetUserLite.AllowMentions {
				case 0:
					isCommunicationAllowed = true
				case 1:
					isCommunicationAllowed = (relationState >= 1)
				case 2:
					isCommunicationAllowed = (relationState == 2)
				}
			default:
				isCommunicationAllowed = true
			}

			if !isCommunicationAllowed {
				continue
			}

			// ── ÉTAPE 4 : HYDRATATION FINALE (Présence et Média) ────────────

			isUserOnline := cache_service.IsUserOnline(ctx, targetUserLite.ID)
			if !targetUserLite.ShowOnlineStatus {
				isUserOnline = false // Règle de confidentialité respectée
			}

			var userAvatarView media_models.MediaView
			if targetUserLite.ProfilePictureID > 0 {
				if mediaView, errMedia := media_service.GenerateMediaViewCascade(ctx, targetUserLite.ProfilePictureID, targetUserLite.ID, 0, callerID); errMedia == nil {
					userAvatarView = mediaView
				}
			}

			validatedUsersList = append(validatedUsersList, auth_models.UserLiteView{
				User:     targetUserLite,
				Avatar:   userAvatarView,
				IsOnline: isUserOnline,
			})
		}

		if input.Query != "" {
			// Les recherches textuelles complètes n'ont pas besoin d'itérer sur des offsets,
			// Redis retourne déjà le meilleur match global.
			break
		}

		currentOffset += fetchBatchLimit
		maxFetchIterations--
	}

	if validatedUsersList == nil {
		validatedUsersList = make([]auth_models.UserLiteView, 0)
	}

	return conversation_models.SuggestOutput{Users: validatedUsersList}, nil
}
