package conversation_service

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/vmihailenco/msgpack/v5"
)

type Candidate struct {
	UserID   int64
	Role     int
	JoinedAt int64
	Relation int
}

// GetConversationAvatars calcule les avatars à afficher pour une conversation en respectant la hiérarchie sociale
func GetConversationAvatars(ctx context.Context, convID int64, callerID int64, convType int) []media_models.MediaView {
	var avatars []media_models.MediaView

	// 1. Récupération des participants en O(1)
	participantsStr, err := redis.ConvParticipants.SMembers(ctx, convID)
	if err != nil || len(participantsStr) == 0 {
		return avatars
	}

	var participantIDs []int64
	for _, pStr := range participantsStr {
		if id, err := strconv.ParseInt(pStr, 10, 64); err == nil {
			participantIDs = append(participantIDs, id)
		}
	}

	// ========================================================================
	// CAS A : Message Privé -> Image du correspondant
	// ========================================================================
	if convType == 0 && len(participantIDs) == 2 {
		for _, pID := range participantIDs {
			if pID != callerID {
				return fetchAvatarsForUsers(ctx, []int64{pID}, convID, callerID)
			}
		}
	}

	// ========================================================================
	// CAS D : Groupe avec seulement 2 personnes -> Image des deux membres (même l'appelant)
	// ========================================================================
	if convType > 0 && len(participantIDs) == 2 {
		return fetchAvatarsForUsers(ctx, participantIDs, convID, callerID)
	}

	// ========================================================================
	// CAS B & C : Groupes et Communautés avec 3+ membres (Tri Complexe RAM)
	// ========================================================================

	// A. MGET sur ConvMembers pour l'ancienneté et le rôle
	var memberKeys []any
	for _, pID := range participantIDs {
		memberKeys = append(memberKeys, fmt.Sprintf("%d:%d", convID, pID))
	}
	membersValues, _ := redis.ConvMembers.MGet(ctx, memberKeys...)

	// B. Création de la liste des candidats et identification du propriétaire
	var candidates []Candidate
	var ownerID int64

	for _, val := range membersValues {
		if val == nil {
			continue
		}
		if strVal, ok := val.(string); ok {
			var mem lite_models.MemberLiteRequest
			if msgpack.Unmarshal([]byte(strVal), &mem) == nil {
				if mem.Role == 2 {
					ownerID = mem.UserID
				}

				// On récupère la relation (Ami > Abonné > Rien) en utilisant ton service existant.
				// Même sans Pipeline, on gagne en propreté, et sur 50 membres ça prendra < 1ms.
				relation := cache_service.RelationValue(ctx, mem.UserID, callerID)

				candidates = append(candidates, Candidate{
					UserID:   mem.UserID,
					Role:     mem.Role,
					JoinedAt: mem.JoinedAt,
					Relation: relation,
				})
			}
		}
	}

	// C. TRI EN RAM (Relation DESC, puis JoinedAt ASC)
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Relation != candidates[j].Relation {
			return candidates[i].Relation > candidates[j].Relation // Le plus grand en premier (2 > 1 > 0)
		}
		return candidates[i].JoinedAt < candidates[j].JoinedAt // Le plus ancien en premier
	})

	// D. SÉLECTION DES GAGNANTS
	var selectedIDs []int64

	if callerID == ownerID {
		// CAS C : L'utilisateur est le propriétaire -> Renvoyer 2 images (les 2 premiers du classement hors lui-même)
		for _, c := range candidates {
			if c.UserID != callerID {
				selectedIDs = append(selectedIDs, c.UserID)
			}
			if len(selectedIDs) == 2 {
				break
			}
		}
	} else {
		// CAS B : L'utilisateur n'est pas le propriétaire -> Renvoyer Proprio + 1 image du classement
		if ownerID != 0 {
			selectedIDs = append(selectedIDs, ownerID)
		}

		for _, c := range candidates {
			// Le membre retenu ne doit être ni le proprio ni l'appelant
			if c.UserID != ownerID && c.UserID != callerID {
				selectedIDs = append(selectedIDs, c.UserID)
				break // On n'en veut qu'un seul
			}
		}
	}

	return fetchAvatarsForUsers(ctx, selectedIDs, convID, callerID)
}

// fetchAvatarsForUsers résout l'Object Cache Média et signe le tout en un éclair
func fetchAvatarsForUsers(ctx context.Context, userIDs []int64, convID int64, callerID int64) []media_models.MediaView {
	var avatars []media_models.MediaView
	if len(userIDs) == 0 {
		return avatars
	}

	getRes, err := redis.UsersLite.GetMany(ctx, userIDs)
	if err != nil {
		return avatars
	}

	// On boucle sur userIDs pour conserver l'ordre du classement
	for _, uID := range userIDs {
		if data, ok := getRes.Found[uID]; ok {
			var u lite_models.UserLiteRequest
			if msgpack.Unmarshal(data, &u) == nil && u.ProfilePictureID > 0 {
				// Signature instantanée via le domaine Média
				if view, err := media_service.GenerateMediaViewCascade(ctx, u.ProfilePictureID, u.ID, convID, callerID); err == nil {
					avatars = append(avatars, view)
				}
			}
		}
	}

	// Evite le retour null
	if avatars == nil {
		avatars = make([]media_models.MediaView, 0)
	}
	return avatars
}
