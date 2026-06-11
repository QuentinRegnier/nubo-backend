package feed_service

import (
	"context"
	"errors"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/feed_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/service/algorithm_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/post_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// GetFeed orchestre la distribution, la rotation (A/B/C), l'élargissement et l'hydratation des posts
func GetFeed(ctx context.Context, input feed_models.GetFeedInput) ([]post_models.GetPostOutput, int, string, error) {
	// 1. Configuration des options du Distributeur sans écraser sournoisement l'index de lecture réel
	isForceTriggered := input.Force

	// 2. Récupération des relations (Amis) en O(1) via le Speed Cache
	friendIDs, _ := cache_service.GetSpeedFriends(ctx, input.UserID)
	friendMap := make(map[int64]bool, len(friendIDs))
	for _, id := range friendIDs {
		friendMap[id] = true
	}

	// Configuration du contexte pour l'algorithme
	opts := algorithm_service.RefreshOptions{
		UserID:        input.UserID,
		LastSeenIndex: input.LastSeenIndex,
		Quotas: algorithm_service.Quotas{
			MaxCandidates: variables.TDDCandidates,
			SocialRatio:   0.3,
			TagRatio:      0.5,
			GlobalRatio:   0.2,
		},
		PersonalOpts: algorithm_service.PersonalizedFeedOptions{
			UserID:         input.UserID,
			UserVec:        nil, // TODO: Profile Service (Architecture Microservice / Edge en attente)
			UserConfidence: 1.0,
			FriendIDs:      friendMap, // ✅ Connexion du Speed Cache pour le paramètre B(u,p)
			Date:           time.Now(),
			Limit:          variables.TDDFeedSize,
		},
		IsForce: isForceTriggered,
	}

	// 3. Boucle d'hydratation sécurisée (Garantit 50 posts stricts malgré les trous de visibilité)
	var validPosts []post_models.GetPostOutput
	needed := variables.FeedPageSize // Ex: 50
	currentIndex := input.LastSeenIndex

	for needed > 0 {
		opts.LastSeenIndex = currentIndex
		opts.FetchCount = needed // On ne demande au Distributeur QUE ce qu'il nous manque

		// Appel au Distributeur (Slice dynamique, et Fusion/Re-sélection si nécessaire)
		idsToFetch, err := algorithm_service.HandlePullToRefresh(ctx, opts)
		if err != nil || len(idsToFetch) == 0 {
			break // Base de données épuisée, on arrête de chercher
		}

		// 4. Extraction de la donnée riche
		postsOutput := post_service.GetPosts(ctx, post_models.GetPostInput{
			UserID:  input.UserID,
			PostIDs: idsToFetch,
		})

		// Filtrage absolu des posts inaccessibles (visibilité privée, suppressions, erreurs Média)
		for _, p := range postsOutput {
			if p.Error == "" && p.Data != nil {
				validPosts = append(validPosts, p)
			}
		}

		// Avancement du curseur global (on a "consommé" len(idsToFetch) index dans le cache Redis)
		currentIndex += len(idsToFetch)
		needed = variables.FeedPageSize - len(validPosts)
	}

	if len(validPosts) == 0 {
		return []post_models.GetPostOutput{}, input.LastSeenIndex, "A", errors.New("aucun post disponible ou visible")
	}

	// 5. Finalisation des métadonnées de pagination
	state, _ := algorithm_service.GetUserFeedState(ctx, input.UserID)
	return validPosts, currentIndex, state.ActiveFeed, nil
}
