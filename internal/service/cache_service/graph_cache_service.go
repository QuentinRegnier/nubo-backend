package cache_service

import (
	"context"
	"log"
	"math"
	"strings"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
	"github.com/vmihailenco/msgpack/v5"
)

// TagEdge représente la valeur binaire stockée dans le HASH Redis
type TagEdge struct {
	Weight    float64 `msgpack:"w"` // Poids W de l'arête
	Timestamp int64   `msgpack:"t"` // Timestamp de la dernière occurrence (ms)
}

// UpdateTagCooccurrences calcule et met à jour les poids markoviens pour toutes les paires de tags d'un post
func UpdateTagCooccurrences(ctx context.Context, tags []string, nowMs int64) {
	if len(tags) < 2 {
		return // Pas de co-occurrence possible avec moins de 2 tags
	}

	// Nettoyage et uniformisation des tags
	var cleanTags []string
	for _, t := range tags {
		clean := strings.ToLower(strings.TrimSpace(t))
		if clean != "" {
			cleanTags = append(cleanTags, clean)
		}
	}

	// On génère toutes les paires uniques (Graphe non orienté)
	for i := 0; i < len(cleanTags); i++ {
		for j := i + 1; j < len(cleanTags); j++ {
			tagA := cleanTags[i]
			tagB := cleanTags[j]

			// Mise à jour A -> B
			updateEdge(ctx, tagA, tagB, nowMs)
			// Mise à jour B -> A (Symétrie pour lecture O(1) depuis n'importe quel sommet)
			updateEdge(ctx, tagB, tagA, nowMs)
		}
	}
}

// updateEdge exécute l'équation d'état : W_nouveau = (W_ancien * e^(-λ * Δt)) * (1 - α) + α
func updateEdge(ctx context.Context, sourceTag, target string, nowMs int64) {
	var edge TagEdge

	// 1. Lecture de l'état actuel via la Collection (encapsulation L1)
	val, err := redis.GraphEdges.HGet(ctx, sourceTag, target).Bytes()
	if err == nil {
		_ = msgpack.Unmarshal(val, &edge)
	}

	// 2. Calcul du Δt en jours
	var deltaDays float64 = 0
	if edge.Timestamp > 0 {
		deltaDays = float64(nowMs-edge.Timestamp) / (1000 * 60 * 60 * 24)
		if deltaDays < 0 {
			deltaDays = 0 // Sécurité anti-voyage dans le temps
		}
	}

	// 3. Moteur Mathématique
	decayedWeight := edge.Weight * math.Exp(-variables.GraphDecayLambda*deltaDays)
	newWeight := decayedWeight*(1.0-variables.GraphLearningAlpha) + variables.GraphLearningAlpha

	// 4. Sauvegarde
	edge.Weight = newWeight
	edge.Timestamp = nowMs
	binData, _ := msgpack.Marshal(&edge)

	_ = redis.GraphEdges.HSet(ctx, sourceTag, target, binData)
}

// GetRelatedTagsLazy retourne les tags cousins et déclenche l'élagage paresseux (Lazy Pruning)
func GetRelatedTagsLazy(ctx context.Context, sourceTag string) map[string]float64 {
	sourceTag = strings.ToLower(strings.TrimSpace(sourceTag))

	edgesJSON, err := redis.GraphEdges.HGetAll(ctx, sourceTag).Result()
	if err != nil || len(edgesJSON) == 0 {
		return nil
	}

	validEdges := make(map[string]float64)
	var edgesToDelete []string
	nowMs := time.Now().UnixMilli()

	// Évaluation à la volée
	for targetTag, binData := range edgesJSON {
		var edge TagEdge
		if err := msgpack.Unmarshal([]byte(binData), &edge); err != nil {
			continue
		}

		deltaDays := float64(nowMs-edge.Timestamp) / (1000 * 60 * 60 * 24)
		currentWeight := edge.Weight * math.Exp(-variables.GraphDecayLambda*deltaDays)

		if currentWeight < variables.GraphSurvivalEps {
			// Le lien est mort (En dessous de ε)
			edgesToDelete = append(edgesToDelete, targetTag)
		} else {
			// Le lien survit
			validEdges[targetTag] = currentWeight
		}
	}

	// 🧹 HDEL Asynchrone : Auto-nettoyage sans bloquer la requête utilisateur
	if len(edgesToDelete) > 0 {
		go func(targets []string) {
			_ = redis.GraphEdges.HDel(context.Background(), sourceTag, targets...)
		}(edgesToDelete)
	}

	return validEdges
}

// SeedGraphCache réalise le Cold Start (Time-Travel Ingestion) de l'écosystème sémantique
func SeedGraphCache(ctx context.Context) error {
	log.Println("🌱 [Graph Cache] Début de l'initialisation du graphe sémantique...")

	// 1. Récupération de l'historique complet, filtré (> 1 tag) et trié (ASC)
	posts, err := postgres.FuncLoadPostsForGraphSeeding(ctx)
	if err != nil {
		return err
	}

	log.Printf("⏳ [Graph Cache] %d publications trouvées pour le rejeu temporel.", len(posts))

	// 2. Rejeu Temporel
	// La boucle va exécuter les équations de Markov en simulant le temps qui passe
	for _, p := range posts {
		// On transmet le timestamp exact de la création du post (Pas l'heure actuelle !)
		UpdateTagCooccurrences(ctx, p.Hashtags, p.CreatedAt.UnixMilli())
	}

	log.Println("✅ [Graph Cache] Graphe sémantique initialisé avec succès !")

	// Note : L'élagage (Pruning) se fera naturellement lors des premières recherches (Lazy Pruning)
	return nil
}
