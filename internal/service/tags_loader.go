package service

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
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
	defer file.Close()

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

// GetTagFromKeyword cherche si un mot (hashtag) correspond à un Tag officiel.
// Retourne le slug du tag (ex: "dev") et true, ou "" et false.
func GetTagFromKeyword(input string) (string, bool) {
	configMutex.RLock() // Lecture seule, pas de blocage excessif
	defer configMutex.RUnlock()

	if keywordMap == nil {
		return "", false
	}

	// Nettoyage basique (minuscule + trim)
	cleanInput := strings.ToLower(strings.TrimSpace(input))
	// Enlève le '#' si présent
	cleanInput = strings.TrimPrefix(cleanInput, "#")

	slug, found := keywordMap[cleanInput]
	return slug, found
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
