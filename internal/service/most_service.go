package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// --- Constantes de Limites (Capping) ---
const (
	MaxTagElements       = 5000 // On ne garde que les 5000 posts les plus hauts par tag
	MaxRankElements      = 5000 // On ne garde que le top 5000 (Likes, Vues, Global, Recent)
	MaxUserPostsElements = 100  // On ne garde que les 100 derniers posts d'un utilisateur en RAM
)

// ============================================================================
// 1. GESTION DE L'ALGORITHME DE RECOMMANDATION (Tags, Global, Recent)
// ============================================================================

// UpdatePostRecommendationScore calcule le score algorithmique du post et
// met à jour sa position dans TOUS les classements de découverte.
// Cette fonction est appelée à la création du post ET à chaque nouvelle interaction (like, vue).
func UpdatePostRecommendationScore(ctx context.Context, postID int64, hashtags []string) {
	// 1. Calcul du nouveau score (via notre moteur de recommandation "bouchon")
	// Plus tard, on pourra passer des bonus spécifiques ici.
	score := CalculateRecommendationScore(postID, nil)
	scoreBoostRecent := CalculateRecommendationScore(postID, nil) // Modification necessary !!!

	// 2. Mise à jour dans le classement GLOBAL (Pour le feed "Pour Toi / Explorer")
	globalKey := "rank:global"
	_ = redis.ZAdd(ctx, globalKey, score, postID)
	_ = redis.ZRemRangeByRank(ctx, globalKey, 0, -(MaxRankElements + 1))

	// 3. Mise à jour dans le classement RECENT (Découverte Fraîcheur)
	// (Plus tard, on appliquera un boost temporel spécifique dans le calcul du score pour cette clé)
	recentKey := "rank:recent"
	_ = redis.ZAdd(ctx, recentKey, scoreBoostRecent, postID)
	_ = redis.ZRemRangeByRank(ctx, recentKey, 0, -(MaxRankElements + 1))

	// 4. Mise à jour dans les TAGS spécifiques (Le post monte ou descend dans son propre tag)
	if len(hashtags) > 0 {
		officialTags := make(map[string]bool)
		for _, hashtag := range hashtags {
			if slug, found := GetTagFromKeyword(hashtag); found {
				officialTags[slug] = true
			}
		}

		for slug := range officialTags {
			tagKey := fmt.Sprintf("idx:tag:%s", slug)
			_ = redis.ZAdd(ctx, tagKey, score, postID)
			_ = redis.ZRemRangeByRank(ctx, tagKey, 0, -(MaxTagElements + 1))
		}
	}
}

// ============================================================================
// 2. GESTION DES CLASSEMENTS PURS (Compteurs : Likes, Views)
// ============================================================================

// IncrementPostMetric augmente le compteur brut d'un post (ex: "likes", "views").
// C'est de la compétition pure (Score = Quantité).
func IncrementPostMetric(ctx context.Context, postID int64, metric string) {
	key := fmt.Sprintf("rank:%s:global", metric)

	// 1. Incrémenter le score (+1)
	err := redis.ZIncrBy(ctx, key, 1.0, postID)
	if err != nil {
		log.Printf("⚠️ Erreur ZIncrBy Rank %s: %v", metric, err)
		return
	}

	// 2. Appliquer la limite stricte (Capping)
	// On supprime du rang 0 (les moins likés) au rang -5001
	err = redis.ZRemRangeByRank(ctx, key, 0, -(MaxRankElements + 1))
	if err != nil {
		log.Printf("⚠️ Erreur Capping Rank %s: %v", metric, err)
	}
}

// ============================================================================
// 3. GESTION DU PROFIL UTILISATEUR (Chronologique strict)
// ============================================================================

// AddPostToUserProfile ajoute un post à la grille rapide du profil utilisateur.
// L'algorithme n'intervient pas ici : on veut un tri chronologique strict.
func AddPostToUserProfile(ctx context.Context, userID int64, postID int64) {
	key := fmt.Sprintf("user:posts:%d", userID)

	// Le score est purement basé sur le temps
	score := float64(time.Now().UnixMilli())

	// 1. Ajouter le post
	err := redis.ZAdd(ctx, key, score, postID)
	if err != nil {
		log.Printf("⚠️ Erreur ZAdd UserProfile %d: %v", userID, err)
		return
	}

	// 2. Appliquer la limite stricte (Capping à 100)
	err = redis.ZRemRangeByRank(ctx, key, 0, -(MaxUserPostsElements + 1))
	if err != nil {
		log.Printf("⚠️ Erreur Capping UserProfile %d: %v", userID, err)
	}
}

// ============================================================================
// 4. LECTURE & PAGINATION (Le Read-Path avec Fallback)
// ============================================================================

// GetUserProfilePosts récupère la grille d'un profil.
func GetUserProfilePosts(ctx context.Context, userID int64, offset int64, limit int64) ([]domain.PostRequest, error) {
	// 1. Fallback Cold Storage (Si on dépasse les 100 derniers posts)
	if offset >= MaxUserPostsElements {
		log.Printf("🧊 [COLD STORAGE] Lecture Mongo directe pour le profil %d (offset: %d)", userID, offset)
		return getPostsFromMongoPaginated("user_id", userID, offset, limit)
	}

	// 2. Lecture RAM (MOST Cache)
	key := fmt.Sprintf("user:posts:%d", userID)
	return fetchAndHydrateFromZSET(ctx, key, offset, limit)
}

// GetRankedPosts récupère un classement (global, recent, likes, views).
func GetRankedPosts(ctx context.Context, rankType string, offset int64, limit int64) ([]domain.PostRequest, error) {
	if offset >= MaxRankElements {
		log.Printf("🧊 [COLD STORAGE] Lecture Mongo directe pour le rank %s (offset: %d)", rankType, offset)

		// Filtre vide : on cherche parmi tous les posts existants
		filter := map[string]any{}

		// Tri par défaut : du plus récent au plus ancien
		sort := map[string]any{"created_at": -1}

		// On interroge Mongo avec la nouvelle fonction paginée
		docs, err := mongo.Posts.GetPaginated(filter, sort, offset, limit)
		if err != nil {
			log.Printf("⚠️ Erreur fallback Mongo RankedPosts: %v", err)
			return []domain.PostRequest{}, err
		}

		var posts []domain.PostRequest
		for _, doc := range docs {
			var p domain.PostRequest
			if err := pkg.ToStruct(doc, &p); err == nil {
				posts = append(posts, p)
			}
		}
		return posts, nil
	}

	key := fmt.Sprintf("rank:%s", rankType)
	return fetchAndHydrateFromZSET(ctx, key, offset, limit)
}

// GetTagPosts récupère la timeline d'un hashtag officiel.
func GetTagPosts(ctx context.Context, slug string, offset int64, limit int64) ([]domain.PostRequest, error) {
	if offset >= MaxTagElements {
		log.Printf("🧊 [COLD STORAGE] Lecture Mongo directe pour le tag %s", slug)
		// Requête Mongo: chercher le slug dans le tableau 'hashtags'
		return getPostsFromMongoPaginated("hashtags", slug, offset, limit)
	}

	key := fmt.Sprintf("idx:tag:%s", slug)
	return fetchAndHydrateFromZSET(ctx, key, offset, limit)
}

// --- FONCTIONS UTILITAIRES PRIVÉES ---

// fetchAndHydrateFromZSET mutualise la logique Redis ZSET -> Hydratation Object Cache
func fetchAndHydrateFromZSET(ctx context.Context, key string, offset int64, limit int64) ([]domain.PostRequest, error) {
	// 1. Récupération des IDs triés
	// Note: offset+limit-1 car Redis ZREVRANGE est inclusif (0 à 19 = 20 éléments)
	idStrings, err := redis.ZRevRange(ctx, key, offset, offset+limit-1)
	if err != nil {
		return nil, fmt.Errorf("erreur lecture ZSET %s: %v", key, err)
	}

	if len(idStrings) == 0 {
		return []domain.PostRequest{}, nil
	}

	// 2. Conversion []string -> []int64
	var ids []int64
	for _, idStr := range idStrings {
		var id int64
		fmt.Sscanf(idStr, "%d", &id)
		ids = append(ids, id)
	}

	// 3. Hydratation via l'Object Cache (Le "Pipeline V2")
	return GetPostsView(ids)
}

// getPostsFromMongoPaginated interroge directement MongoDB quand le client scrolle trop loin.
// Remplace le mock par la vraie logique.
func getPostsFromMongoPaginated(field string, value any, offset int64, limit int64) ([]domain.PostRequest, error) {
	filter := map[string]any{
		field: value,
	}

	// Tri chronologique par défaut (le plus récent en premier)
	sort := map[string]any{"created_at": -1}

	// Appel de la fonction que l'on vient d'ajouter dans generic.go
	docs, err := mongo.Posts.GetPaginated(filter, sort, offset, limit)
	if err != nil {
		log.Printf("⚠️ Erreur fallback Mongo paginé: %v", err)
		return []domain.PostRequest{}, err
	}

	// Conversion des map[string]any brutes en domain.PostRequest
	var posts []domain.PostRequest
	for _, doc := range docs {
		var p domain.PostRequest
		if err := pkg.ToStruct(doc, &p); err == nil {
			posts = append(posts, p)
		}
	}

	return posts, nil
}
