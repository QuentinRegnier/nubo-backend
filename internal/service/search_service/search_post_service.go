package search_service

import (
	"context"
	"strings"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/search_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/post_service"
)

// SearchPosts orchestre la recherche complexe de publications.
func SearchPosts(ctx context.Context, callerID int64, input search_models.SearchPostInput) (search_models.SearchPostOutput, error) {
	output := search_models.SearchPostOutput{
		Posts:                   make([]post_models.GetPostOutput, 0),
		HasTrendFilterAvailable: false,
	}

	limit := input.Limit
	if limit == 0 {
		limit = 20
	}
	filter := input.Filter
	if filter == "" {
		filter = "views" // Filtre par défaut selon tes specs
	}

	query := strings.TrimSpace(input.Query)
	var postIDs []int64

	// Mapping du filtre en entier pour Postgres
	orderModeMap := map[string]int{
		"views":    0,
		"likes":    1,
		"comments": 2,
		"recent":   3,
		"oldest":   4,
	}

	// =========================================================================
	// CAS 1 : RECHERCHE PAR TAG (La query commence par #)
	// =========================================================================
	if strings.HasPrefix(query, "#") {
		cleanTag := service.NormalizeHashtag(query[1:]) // On retire le # et on normalise
		if cleanTag == "" {
			return output, nil
		}

		// Vérification du badge "Trend" en O(1) RAM
		output.HasTrendFilterAvailable = cache_service.IsTagTrending(ctx, cleanTag)

		// Si l'utilisateur a demandé le filtre "trend" et qu'il y a droit
		if filter == "trend" && output.HasTrendFilterAvailable {
			// On tape directement dans le ZSET algorithmique pré-calculé (Ultra rapide)
			trendPosts, err := cache_service.GetPostsByTagFromCache(ctx, cleanTag, input.Offset, limit)
			if err == nil {
				for _, p := range trendPosts {
					postIDs = append(postIDs, p.ID)
				}
			}
		} else {
			// Sinon, on tape dans Postgres avec le tri classique demandé
			orderMode := orderModeMap[filter]
			postIDs, _ = postgres.FuncSearchPostIDsByTag(ctx, cleanTag, orderMode, input.Offset, limit)
		}

	} else {
		// =========================================================================
		// CAS 2 : RECHERCHE PAR UTILISATEURS (Texte classique)
		// =========================================================================
		// A. On régénère le panier d'utilisateurs instantanément en RAM
		liteUsers, _ := cache_service.SearchUserByPrefix(ctx, query, 15) // Max 15 profils pour le panier
		if len(liteUsers) == 0 {
			return output, nil
		}

		var userIDs []int64
		for _, u := range liteUsers {
			userIDs = append(userIDs, u.ID)
		}

		// B. On cherche leurs posts combinés dans Postgres avec le tri
		orderMode := orderModeMap[filter]
		if filter == "trend" {
			orderMode = 0 // Pas de trend global ici, on fallback sur les vues
		}

		postIDs, _ = postgres.FuncSearchPostIDsByUsers(ctx, userIDs, orderMode, input.Offset, limit)
	}

	// =========================================================================
	// RÉSULTAT ET HYDRATATION GLOBALE
	// =========================================================================
	if len(postIDs) == 0 {
		return output, nil
	}

	// On délègue à post_service.GetPosts : Il gère TOUTE la sécurité (visibilité, ban, URL signées)
	rawPosts := post_service.GetPosts(ctx, post_models.GetPostInput{
		UserID:  callerID,
		PostIDs: postIDs,
	})

	// On purge les posts inaccessibles (GetPosts met Error != "" si interdit)
	for _, p := range rawPosts {
		if p.Error == "" {
			output.Posts = append(output.Posts, p)
		}
	}

	return output, nil
}
