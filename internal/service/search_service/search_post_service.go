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

// Helper pour mapper la string du front-end en entier pour le SQL
func mapFilterToOrderMode(filter string) int {
	switch filter {
	case "views":
		return 0
	case "likes":
		return 1
	case "comments":
		return 2
	case "recent":
		return 3
	case "oldest":
		return 4
	default:
		return 3 // recent par défaut
	}
}

func SearchPosts(ctx context.Context, callerID int64, input search_models.SearchPostInput) (search_models.SearchPostOutput, error) {
	var postIDs []int64
	intercepted := false
	orderMode := mapFilterToOrderMode(input.Filter)

	// 1. DÉCOUVERTE GLOBALE (Filtre seul, sans recherche textuelle) -> MOST CACHE L1
	if input.Query == "" && input.Filter != "" && input.Filter != "trend" {
		rankType := ""
		switch input.Filter {
		case "likes":
			rankType = "strict:likes"
		case "views":
			rankType = "strict:views"
		case "recent":
			rankType = "strict:recent"
		}

		if rankType != "" {
			payloads, err := cache_service.GetRankedPosts(ctx, rankType, input.Offset, input.Limit)
			if err != nil {
				return search_models.SearchPostOutput{}, err
			}
			for _, p := range payloads {
				postIDs = append(postIDs, p.ID)
			}
			intercepted = true
		}
	}

	// 2. RECHERCHE PAR HASHTAG EXACT (#)
	if !intercepted && strings.HasPrefix(input.Query, "#") {
		cleanSlug := service.NormalizeHashtag(input.Query)
		if cleanSlug != "" {
			// Si on demande les plus récents, on tape le ZSET ultra-rapide L1
			if orderMode == 3 {
				payloads, err := cache_service.GetTagPosts(ctx, cleanSlug, input.Offset, input.Limit)
				if err == nil {
					for _, p := range payloads {
						postIDs = append(postIDs, p.ID)
					}
					intercepted = true
				}
			} else {
				// Si on veut trier (par ex: "Le tag #nature le plus liké"), on DOIT déléguer au L3
				pgIDs, errPg := postgres.FuncSearchPostIDsByTag(ctx, cleanSlug, orderMode, input.Offset, input.Limit)
				if errPg == nil {
					postIDs = append(postIDs, pgIDs...)
					intercepted = true
				}
			}
		}
	}

	// 3. RECHERCHE PAR AUTEUR EXACT (@)
	if !intercepted && strings.HasPrefix(input.Query, "@") {
		username := strings.ToLower(strings.TrimPrefix(input.Query, "@"))
		// Recherche instantanée L1 Lex pour trouver l'ID
		users, err := cache_service.SearchUserByPrefix(ctx, username, 1)
		if err == nil && len(users) > 0 && strings.ToLower(users[0].Username) == username {
			pgIDs, errPg := postgres.FuncSearchPostIDsByUsers(ctx, []int64{users[0].ID}, orderMode, input.Offset, input.Limit)
			if errPg == nil {
				postIDs = append(postIDs, pgIDs...)
				intercepted = true
			}
		}
	}

	// 4. FALLBACK : RECHERCHE TEXTUELLE COMPLÈTE (L3 Postgres)
	if !intercepted && input.Query != "" {
		pgIDs, errPg := postgres.FuncSearchPostIDsByText(ctx, input.Query, orderMode, input.Offset, input.Limit)
		if errPg != nil {
			return search_models.SearchPostOutput{}, errPg
		}
		postIDs = append(postIDs, pgIDs...)
	}

	// 5. HYDRATATION FINALE UNIFIÉE (L1 -> L2 -> L3)
	// Si on n'a trouvé aucun ID à cette étape, on renvoie une liste vide propre
	if len(postIDs) == 0 {
		return search_models.SearchPostOutput{
			Posts:                   []post_models.GetPostOutput{},
			HasTrendFilterAvailable: true,
		}, nil
	}

	hydratedPosts := post_service.GetPosts(ctx, post_models.GetPostInput{
		UserID:  callerID,
		PostIDs: postIDs,
	})

	return search_models.SearchPostOutput{
		Posts:                   hydratedPosts,
		HasTrendFilterAvailable: true,
	}, nil
}
