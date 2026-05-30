package cache_service

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// ============================================================================
// 3. ROUTINES PRIVÉES D'ACCÈS AUX DONNÉES
// ============================================================================

func fetchAndHydrateFromZSET(ctx context.Context, key string, offset int64, limit int64) ([]models.PostRequest, error) {
	idStrings, err := redis.ZRevRange(ctx, key, offset, offset+limit-1)
	if err != nil {
		return nil, fmt.Errorf("erreur lecture ZSET %s: %w", key, err)
	}

	if len(idStrings) == 0 {
		return []models.PostRequest{}, nil
	}

	var ids []int64
	for _, idStr := range idStrings {
		var id int64
		_, err := fmt.Sscanf(idStr, "%d", &id)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return GetPostsView(ids)
}

func getPostsFromMongoPaginated(field string, value any, offset int64, limit int64) ([]models.PostRequest, error) {
	filter := map[string]any{field: value}
	sort := map[string]any{"created_at": -1}

	docs, err := mongo.Posts.GetPaginated(filter, sort, offset, limit)
	if err != nil {
		return []models.PostRequest{}, err
	}

	var posts []models.PostRequest
	for _, doc := range docs {
		var p models.PostRequest
		if err := pkg.ToStruct(doc, &p); err == nil {
			posts = append(posts, p)
		}
	}

	return posts, nil
}

func getPostsFromPostgresPaginated(ctx context.Context, rankType string, offset int64, limit int64) ([]models.PostRequest, error) {
	// TODO: Optimiser ces requêtes avec des vues matérialisées si la BDD dépasse 1M de lignes
	var query string

	switch rankType {
	case "likes:strict":
		query = `
			SELECT p.id FROM content.posts p 
			WHERE p.visibility != 2 
			ORDER BY (SELECT COUNT(*) FROM content.likes l WHERE l.target_id = p.id AND l.target_type = 0) DESC, p.created_at DESC 
			OFFSET $1 LIMIT $2`
	case "views:strict":
		query = `
			SELECT p.id FROM content.posts p 
			WHERE p.visibility != 2 
			ORDER BY (SELECT COUNT(*) FROM content.views v WHERE v.target_id = p.id AND v.target_type = 0) DESC, p.created_at DESC 
			OFFSET $1 LIMIT $2`
	default:
		query = `SELECT id FROM content.posts WHERE visibility != 2 ORDER BY created_at DESC OFFSET $1 LIMIT $2`
	}

	rows, err := postgres.PostgresDB.QueryContext(ctx, query, offset, limit)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Printf("⚠️ Erreur fermeture rows L3 Postgres paginé: %v", err)
		}
	}(rows)

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}

	return GetPostsView(ids)
}

// ============================================================================
// 4. HYDRATATION FINALE DU FEED (Étape 5.3)
// ============================================================================

// HydrateFeed prend les IDs bruts d'une page de buffer (générée par le Distributeur)
// et les transforme en objets complets prêts pour le frontend.
// C'est ICI que l'on applique le filtrage de dernière minute (Visibility, Banned).
func HydrateFeed(ctx context.Context, postIDs []int64) ([]models.PostRequest, error) {
	// Pré-allocation pour optimiser la mémoire
	hydratedPosts := make([]models.PostRequest, 0, len(postIDs))

	for _, id := range postIDs {
		var post models.PostRequest

		// 1. Tentative de récupération depuis le cache_service LFU (Object Cache Redis)
		// C'est le même cache_service que tu initialises dans CreatePost.
		err := redis.Posts.GetObject(ctx, id, &post)
		if err != nil {
			// FALLBACK : Si le post_service a été évincé du cache_service Redis, on va le chercher en base
			// (En réutilisant ta méthode GetPostsView existante qui tape sur la BDD)
			postsFromDB, errDB := GetPostsView([]int64{id})
			if errDB == nil && len(postsFromDB) > 0 {
				post = postsFromDB[0]
				// Réhydratation silencieuse du cache_service LFU pour les prochains appels
				_ = redis.Posts.SetObject(ctx, id, post)
			} else {
				continue // Post totalement introuvable (Hard Delete), on l'ignore
			}
		}

		// ─────────────────────────────────────────────────────────────────────
		// 2. ÉTAPE 5.3 : GESTION "A LA VOLÉE" DES ÉTATS CRITIQUES
		// ─────────────────────────────────────────────────────────────────────
		// Si le post_service a été modéré, ou l'auteur banni entre la création du buffer et la lecture.
		// On s'aligne sur ta logique Postgres où la visibilité '2' équivaut à un post_service supprimé/masqué.
		if post.Visibility == 2 {
			// Le post_service est ignoré silencieusement côté backend.
			// Le frontend ne le recevra même pas, ce qui économise de la bande passante
			// et garantit qu'aucune donnée d'un utilisateur banni ne fuite.
			continue
		}

		// (Optionnel) : Tu pourrais aussi vérifier le statut de l'auteur ici
		// via un appel ultra-rapide au cache_service Utilisateur si nécessaire.

		hydratedPosts = append(hydratedPosts, post)
	}

	return hydratedPosts, nil
}
