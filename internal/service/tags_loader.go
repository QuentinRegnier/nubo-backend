package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"

	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// --- Structures de Données (Miroir du JSON) ---

type TagsConfig struct {
	Version     string     `json:"version"`
	LastUpdated time.Time  `json:"last_updated"`
	Categories  []Category `json:"categories"`
}

type Category struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Order int    `json:"order"`
	Tags  []Tag  `json:"tags"`
}

type Tag struct {
	Slug     string   `json:"slug"`
	Label    string   `json:"label"`
	Keywords []string `json:"keywords"`
	Icon     string   `json:"icon"`
	Color    string   `json:"color"`
	Priority int      `json:"priority"`
}

// --- Singleton & Variables Globales ---

var (
	// currentConfig stocke la structure brute (pour l'affichage UI)
	currentConfig *TagsConfig

	// keywordMap est un index inversé pour la recherche rapide (O(1))
	// Ex: "golang" -> "dev"
	keywordMap map[string]string

	// Mutex pour gérer le rechargement à chaud sans crash (Thread-Safe)
	configMutex sync.RWMutex
)

// --- Fonctions Publiques ---

// LoadTagsConfig charge le fichier JSON en mémoire et construit les index.
func LoadTagsConfig(filePath string) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	log.Printf("📂 Chargement de la configuration des tags depuis %s...", filePath)

	// 1. Lecture du fichier
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("impossible d'ouvrir le fichier tags: %v", err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Printf("⚠️ Erreur lors de la fermeture du fichier tags: %v", err)
		}
	}(file)

	// 2. Parsing JSON
	var config TagsConfig
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return fmt.Errorf("JSON invalide dans tags_config: %v", err)
	}

	// 3. Construction de l'Index Inversé (Keyword -> Tag Slug)
	// Cela permettra au worker de savoir instantanément que "#btc" = "crypto"
	newKeywordMap := make(map[string]string)
	countTags := 0

	for _, cat := range config.Categories {
		for _, tag := range cat.Tags {
			countTags++
			// On mappe le slug lui-même
			newKeywordMap[strings.ToLower(tag.Slug)] = tag.Slug

			// On mappe tous les mots-clés associés
			for _, kw := range tag.Keywords {
				newKeywordMap[strings.ToLower(kw)] = tag.Slug
			}
		}
	}

	// 4. Mise à jour des pointeurs globaux
	currentConfig = &config
	keywordMap = newKeywordMap

	log.Printf("✅ Tags chargés : %d catégories, %d tags, %d mots-clés indexés.",
		len(config.Categories), countTags, len(newKeywordMap))

	return nil
}

// NormalizeHashtag effectue une normalisation lexicale stricte pour éviter 90% des doublons.
func NormalizeHashtag(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	input = strings.TrimPrefix(input, "#")

	// Retrait des caractères spéciaux : on ne garde que l'alphanumérique
	var builder strings.Builder
	builder.Grow(len(input))
	for _, r := range input {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

// GetTagFromKeyword cherche le slug officiel (canonique) d'un hashtag.
// Désormais, TOUS les hashtags sont valides (renvoie toujours true si non vide).
func GetTagFromKeyword(ctx context.Context, input string) (string, bool) {
	cleanInput := NormalizeHashtag(input)
	if cleanInput == "" {
		return "", false
	}

	// 1. Vérification dans l'index statique prioritaire (tags_config.json)
	configMutex.RLock()
	slug, found := keywordMap[cleanInput]
	configMutex.RUnlock()

	if found {
		return slug, true
	}

	// 2. Vérification dans le mapping dynamique Redis (Fautes de frappe corrigées)
	canonSlug, err := redisgo.Rdb.HGet(ctx, variables.RedisKeyHashtagCanonMap, cleanInput).Result()
	if err == nil && canonSlug != "" {
		return canonSlug, true
	}

	// 3. Si inconnu, le mot propre devient son propre tag (Nouveau Tag Communautaire)
	// On l'ajoute silencieusement au SET des tags actifs pour que le Cron de nuit l'analyse.
	_ = redisgo.Rdb.SAdd(ctx, variables.RedisKeyActiveTagsSet, cleanInput).Err()

	return cleanInput, true
}

// GetTagsConfig retourne la configuration complète (utile pour l'envoyer au Frontend).
func GetTagsConfig() TagsConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()

	if currentConfig == nil {
		return TagsConfig{}
	}
	// On retourne une copie par valeur pour éviter les modifications accidentelles
	return *currentConfig
}
