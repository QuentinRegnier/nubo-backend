package cache_service

import (
	"context"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # SERVICE : GESTION DE LA TIMELINE UTILISATEUR (ZSET)
// ############################################################################

// GetTopUserPostIDs récupère la timeline, gère les profils vides, et prévient
// de manière déterministe les cache misses vers MongoDB/Postgres.
func GetTopUserPostIDs(ctx context.Context, userID int64, offset int64, limit int64) ([]int64, error) {

	// ── ÉTAPE 1 : VÉRIFICATION D'EXISTENCE (ÉVITE LE CACHE MISS BDD) ────────

	isZSetPresent, errExists := redis.UserTimeline.Exists(ctx, userID)
	if errExists != nil {
		logger.Log.Warn().Err(errExists).Msg("Erreur lors de la vérification de l'existence de la UserTimeline")
	}

	if !isZSetPresent {
		// Le retour d'une erreur déclenchera le fallback (L2 -> L3) par la couche appelante
		return nil, nubo_error.NewInternal()
	}

	// ── ÉTAPE 2 : EXTRACTION PAGINÉE DU ZSET (O(log(N)+M)) ──────────────────

	idStringsList, errRedis := redis.UserTimeline.ZRevRange(ctx, userID, offset, offset+limit-1)
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Int64("user_id", userID).Msg("Échec de lecture de la timeline ZSET")
		return nil, nubo_error.NewInternal()
	}

	var parsedPostIDs []int64 // Initialisation propre (slice non-nil)

	for _, idString := range idStringsList {

		// Ignore silencieusement le marqueur de profil vide
		if idString == variables.UserTimelineEmptyMarker {
			continue
		}

		if parsedID, errParse := strconv.ParseInt(idString, 10, 64); errParse == nil {
			parsedPostIDs = append(parsedPostIDs, parsedID)
		}
	}

	return parsedPostIDs, nil
}

// MarkUserTimelineEmpty injecte un marqueur fictif pour sceller le Cache Hit
// et indiquer qu'un utilisateur n'a réellement aucune publication à son actif.
func MarkUserTimelineEmpty(ctx context.Context, userID int64) error {
	errRedis := redis.UserTimeline.ZAdd(ctx, userID, 0, variables.UserTimelineEmptyMarker)
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Msg("Impossible de marquer la timeline comme vide")
		return nubo_error.NewInternal()
	}

	_ = redis.UserTimeline.RefreshTTL(ctx, userID)
	return nil
}

// PurgeUserTimeline détruit totalement le ZSET associé à l'utilisateur.
func PurgeUserTimeline(ctx context.Context, userID int64) error {
	// DeleteObject encapsule le DEL physique de la clé Redis (Pur DDD)
	errRedis := redis.UserTimeline.DeleteObject(ctx, userID)
	if errRedis != nil {
		logger.Log.Warn().Err(errRedis).Msg("Erreur lors de la purge de la UserTimeline")
		return nubo_error.NewInternal()
	}
	return nil
}

// AddPostToUserProfile insère un nouveau post au sommet de la timeline et retire
// automatiquement le marqueur "profil vide" s'il existait.
func AddPostToUserProfile(ctx context.Context, userID int64, postID int64, timestampScore float64) error {

	// Nettoyage préventif
	_ = redis.UserTimeline.ZRem(ctx, userID, variables.UserTimelineEmptyMarker)

	postIDString := strconv.FormatInt(postID, 10)
	errRedis := redis.UserTimeline.ZAdd(ctx, userID, timestampScore, postIDString)
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Msg("Échec de l'ajout du post dans le ZSET utilisateur")
		return nubo_error.NewInternal()
	}

	return nil
}

// RemovePostFromUserProfile expulse l'ID d'une publication de la timeline (ex: Soft Delete).
func RemovePostFromUserProfile(ctx context.Context, userID int64, postID int64) error {
	postIDString := strconv.FormatInt(postID, 10)
	errRedis := redis.UserTimeline.ZRem(ctx, userID, postIDString)
	if errRedis != nil {
		logger.Log.Warn().Err(errRedis).Msg("Échec du retrait du post dans le ZSET utilisateur")
		return nubo_error.NewInternal()
	}
	return nil
}
