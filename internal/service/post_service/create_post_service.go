package post_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/algorithm_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
)

// CreatePost orchestre la publication d'un post.
func CreatePost(ctx context.Context, userID int64, input post_models.CreatePostInput) (int64, error) {
	// ========================================================================
	// 0. BOUCLIER DE CONFIDENTIALITÉ ACTIF (Tags et Mentions)
	// ========================================================================
	// Extraction des IDs mentionnés dans le texte via la regex globale (pkg)
	mentionedIDs := pkg.ExtractMentions(input.Content)

	// Fonction interne de vérification des permissions respectant le DDD et la matrice de relation
	checkPermissions := func(targetIDs []int64, permType string) error {
		for _, tID := range targetIDs {
			if tID == userID {
				continue // S'auto-identifier ou s'auto-mentionner est toujours autorisé
			}

			// Récupération de l'empreinte Speed Cache (L1) de la cible
			tLite, err := cache_service.GetUserLite(ctx, tID)
			if err != nil || tLite.ID == 0 {
				continue // Si l'utilisateur n'existe pas en cache, on laisse passer ou le validateur gérera
			}

			// "Qu'est-ce que la Cible (tID) pense du Demandeur (userID) ?"
			relState := cache_service.RelationValue(ctx, tID, userID)
			if relState == -1 {
				return nubo_error.NewForbidden("USER_BLOCKED", "Action impossible : vous ne pouvez pas identifier un utilisateur qui vous a bloqué.", nil)
			}

			// Sélection de la règle selon qu'il s'agit d'un tag de média ou d'une mention textuelle
			perm := tLite.AllowTagging
			if permType == "mention" {
				perm = tLite.AllowMentions
			}

			allowed := false
			switch perm {
			case 0: // Tout le monde
				allowed = true
			case 1: // Abonnés uniquement (relation >= 1 : Follower ou Ami)
				allowed = (relState >= 1)
			case 2: // Amis uniquement (relation == 2)
				allowed = (relState == 2)
			case 3: // Personne
				allowed = false
			}

			if !allowed {
				return nubo_error.NewForbidden("PRIVACY_RESTRICTION", "Un des utilisateurs identifiés restreint les identifications ou mentions.", nil)
			}
		}
		return nil
	}

	// A. Vérification des tags sur les médias (Identifiers) -> S'appuie sur AllowTagging
	if err := checkPermissions(input.Identifiers, "tag"); err != nil {
		return -1, err
	}

	// B. Vérification des mentions textuelles (@{id}) -> S'appuie sur AllowMentions
	if err := checkPermissions(mentionedIDs, "mention"); err != nil {
		return -1, err
	}

	now := time.Now().UTC()
	postID := pkg.GenerateID()

	// 1. DÉLÉGATION : Activation des médias fantômes
	if err := media_service.ActivateMediaBatch(ctx, input.MediaIDs, userID); err != nil {
		return -1, err
	}

	// 2. DÉLÉGATION : Évaluation dynamique de la priorité
	priorityLevel := 0
	if userLite, err := cache_service.GetUserLite(ctx, userID); err == nil {
		gradeToPriority := map[int]int{
			0: 0, 1: 1, 2: 2, 3: 3, 4: 4,
		}
		if p, ok := gradeToPriority[userLite.Grade]; ok {
			priorityLevel = p
		}
	}

	// 3. ASSEMBLAGE : Création de l'objet Post
	post := post_models.PostPayload{
		ID:                postID,
		UserID:            userID,
		Content:           pkg.CleanStr(input.Content),
		Hashtags:          input.Hashtags,
		IndirectHashtags:  nil, // Initialisation stricte anti-fantôme
		Identifiers:       input.Identifiers,
		MediaIDs:          input.MediaIDs,
		Visibility:        input.Visibility,
		PriorityLevel:     priorityLevel,
		Location:          input.Location,
		CreatedAt:         domain.TimeToMillis(now),
		UpdatedAt:         domain.TimeToMillis(now),
		LikeCount:         0,
		CommentCount:      0,
		ViewCount:         0,
		ReportCount:       0,
		HasMedia:          len(input.MediaIDs) > 0,
		VectorVersion:     1,
		TelemetryDwellSum: 0.0,
		TelemetryDwellSq:  0.0,
		TelemetryClicks:   0,
	}

	// 4. DÉLÉGATION : Vectorisation synchrone du contenu (O(1))
	post.Vector = algorithm_service.ComputeContentVectorFull(post, nil)

	// 5. DÉLÉGATION : Cache Redis (LFU Init) & Timeline
	if err := object_cache_service.SetPostInObjectCache(ctx, post); err != nil {
		return -1, err
	}
	_ = cache_service.AddPostToUserProfile(ctx, userID, postID, float64(now.UnixMilli()))

	// 6. DÉLÉGATION : Persistance Asynchrone (Write-Behind)
	err := redis.EnqueueDB(ctx, postID, 0, redis.EntityPost, redis.ActionCreate, post, redis.TargetAll)

	return postID, err
}
