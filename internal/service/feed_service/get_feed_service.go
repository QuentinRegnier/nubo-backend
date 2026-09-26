package feed_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/feed_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/service/algorithm_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/post_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # ORCHESTRATEUR PRINCIPAL DU FLUX D'ACTUALITÉS (GET FEED)
// ############################################################################

// GetFeed orchestre la distribution, la rotation (A/B/C), l'élargissement et l'hydratation des posts.
func GetFeed(ctx context.Context, input feed_models.GetFeedInput) ([]post_models.GetPostOutput, int, string, error) {

	isManualRefresh := input.Force

	// ── ÉTAPE 1 : Rapatriement du Graphe Social (O(1) L1) ─────────────────
	friendIDsList, _ := cache_service.GetSpeedFriends(ctx, input.UserID)
	friendIDsMap := make(map[int64]bool, len(friendIDsList))
	for _, friendID := range friendIDsList {
		friendIDsMap[friendID] = true
	}

	// ── ÉTAPE 2 : Lecture de l'ADN Algorithmique (Télémétrie L1) ──────────
	userVector, err := cache_service.GetTelemetryVector(ctx, input.UserID)
	if err != nil || len(userVector) != variables.VectorDimTotal {
		// FALLBACK GRACIEUX : Nouvel utilisateur ou cache LFU évincé.
		// Le moteur mathématique basculera automatiquement sur le Trend Global.
		userVector = nil
	}

	// ── ÉTAPE 3 : Paramétrage du Distributeur et du Magasinier ────────────
	refreshOptions := algorithm_service.RefreshOptions{
		UserID:        input.UserID,
		LastSeenIndex: input.LastSeenIndex,
		Quotas: algorithm_service.Quotas{
			MaxCandidates: variables.TDDCandidates,
			SocialRatio:   variables.SocialRatio,
			TagRatio:      variables.TagRatio,
			GlobalRatio:   variables.GlobalRatio,
		},
		PersonalOpts: algorithm_service.PersonalizedFeedOptions{
			UserID:         input.UserID,
			UserVec:        userVector,
			UserConfidence: 1.0, // Réservé pour l'IA embarquée mobile (MLX/CoreML)
			FriendIDs:      friendIDsMap,
			Date:           time.Now(),
			Limit:          variables.TDDFeedSize,
		},
		IsForce: isManualRefresh,
	}

	// ── ÉTAPE 4 : Boucle d'hydratation sécurisée (Remplissage strict) ─────
	var hydratedFeed []post_models.GetPostOutput
	missingPostsCount := variables.FeedPageSize
	currentScrollIndex := input.LastSeenIndex

	// La boucle garantit que le client reçoit exactement le nombre de posts demandés,
	// même si certains posts du cache ont été supprimés ou rendus privés entre-temps.
	for missingPostsCount > 0 {
		refreshOptions.LastSeenIndex = currentScrollIndex
		refreshOptions.FetchCount = missingPostsCount

		// Extraction via les Tampons Tournants A/B/C
		fetchedIDs, err := algorithm_service.HandlePullToRefresh(ctx, refreshOptions)
		if err != nil || len(fetchedIDs) == 0 {
			break // ZSET épuisé, on arrête l'hydratation
		}

		// Hydratation riche (Média HMAC, Commentaires, Pseudos, Visibilité)
		richPostsData := post_service.GetPosts(ctx, post_models.GetPostInput{
			UserID:  input.UserID,
			PostIDs: fetchedIDs,
		})

		// Filtrage absolu des trous de visibilité (Soft Deletes)
		for _, postView := range richPostsData {
			if postView.Error == "" && postView.Data.ID != 0 {
				hydratedFeed = append(hydratedFeed, postView)
			}
		}

		currentScrollIndex += len(fetchedIDs)
		missingPostsCount = variables.FeedPageSize - len(hydratedFeed)
	}

	// ── ÉTAPE 5 : Rendu Client ────────────────────────────────────────────
	if len(hydratedFeed) == 0 {
		return []post_models.GetPostOutput{}, input.LastSeenIndex, "A", nubo_error.NewNotFound(nubo_error.CodeNotFound, "Aucun post disponible ou visible.", nil)
	}

	// Récupération de la lettre du tampon actif (A, B ou C) pour le debug client
	feedState, _ := algorithm_service.GetUserFeedState(ctx, input.UserID)
	return hydratedFeed, currentScrollIndex, feedState.ActiveFeed, nil
}
