package cache_service

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// ############################################################################
// # ROUTINES PRIVÉES D'ACCÈS AUX DONNÉES (HYDRATATION POSTS)
// ############################################################################

// fetchAndHydrateFromCollection abstrait la récupération d'IDs depuis un ZSET Redis
// et lance la cascade d'hydratation L1 -> L2 -> L3.
func fetchAndHydrateFromCollection(ctx context.Context, redisCollection *redis.Collection, targetKey any, offset int64, limit int64) ([]post_models.PostPayload, error) {
	idStringsList, errRedis := redisCollection.ZRevRange(ctx, targetKey, offset, offset+limit-1)
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Msg("Échec de lecture ZSET dans fetchAndHydrateFromCollection")
		return nil, nubo_error.NewInternal()
	}

	if len(idStringsList) == 0 {
		return []post_models.PostPayload{}, nil
	}

	var parsedIDsList []int64
	for _, idString := range idStringsList {
		var parsedID int64
		_, errScan := fmt.Sscanf(idString, "%d", &parsedID)
		if errScan != nil {
			logger.Log.Warn().Err(errScan).Str("id_string", idString).Msg("Erreur de parsing d'ID dans ZSET")
			continue
		}
		parsedIDsList = append(parsedIDsList, parsedID)
	}

	// Déclenchement de la cascade complète
	return object_cache_service.GetPostsView(parsedIDsList)
}

// getPostsFromMongoPaginated interroge directement le Warm Storage MongoDB.
func getPostsFromMongoPaginated(fieldName string, expectedValue any, offset int64, limit int64) ([]post_models.PostPayload, error) {
	mongoFilter := map[string]any{fieldName: expectedValue}
	mongoSort := map[string]any{"created_at": -1}

	mongoDocuments, errMongo := mongo.Posts.GetPaginated(mongoFilter, mongoSort, offset, limit)
	if errMongo != nil {
		logger.Log.Error().Err(errMongo).Msg("Erreur L2 lors de la récupération paginée Mongo")
		return []post_models.PostPayload{}, nubo_error.NewInternal()
	}

	var hydratedPosts []post_models.PostPayload
	for _, document := range mongoDocuments {
		var postPayload post_models.PostPayload
		if errStruct := pkg.ToStruct(document, &postPayload); errStruct == nil {
			hydratedPosts = append(hydratedPosts, postPayload)
		}
	}

	return hydratedPosts, nil
}

// getPostsFromPostgresPaginated est le fallback ultime (Cold Storage) pour les classements.
func getPostsFromPostgresPaginated(ctx context.Context, rankType string, offset int64, limit int64) ([]post_models.PostPayload, error) {
	var sqlQuery string

	// TODO: Optimiser ces requêtes avec des vues matérialisées si la BDD dépasse 1M de lignes
	switch rankType {
	case "likes:strict":
		sqlQuery = `
			SELECT p.id FROM content.posts p 
			WHERE p.visibility != 2 
			ORDER BY (SELECT COUNT(*) FROM content.likes l WHERE l.target_id = p.id AND l.target_type = 0) DESC, p.created_at DESC 
			OFFSET $1 LIMIT $2`
	case "views:strict":
		sqlQuery = `
			SELECT p.id FROM content.posts p 
			WHERE p.visibility != 2 
			ORDER BY (SELECT COUNT(*) FROM content.views v WHERE v.target_id = p.id AND v.target_type = 0) DESC, p.created_at DESC 
			OFFSET $1 LIMIT $2`
	default:
		sqlQuery = `SELECT id FROM content.posts WHERE visibility != 2 ORDER BY created_at DESC OFFSET $1 LIMIT $2`
	}

	sqlRows, errPg := postgres.PostgresDB.QueryContext(ctx, sqlQuery, offset, limit)
	if errPg != nil {
		logger.Log.Error().Err(errPg).Str("rank_type", rankType).Msg("Échec L3 lors de la requête de classement paginée")
		return nil, nubo_error.NewInternal()
	}

	defer func(rowsToClose *sql.Rows) {
		if errClose := rowsToClose.Close(); errClose != nil {
			logger.Log.Warn().Err(errClose).Msg("Erreur lors de la fermeture du curseur Rows L3 Postgres")
		}
	}(sqlRows)

	var extractedIDs []int64
	for sqlRows.Next() {
		var extractedID int64
		if errScan := sqlRows.Scan(&extractedID); errScan == nil {
			extractedIDs = append(extractedIDs, extractedID)
		}
	}

	return object_cache_service.GetPostsView(extractedIDs)
}
