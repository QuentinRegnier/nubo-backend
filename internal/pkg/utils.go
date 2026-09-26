package pkg

import (
	"fmt"
	"html"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
)

// Ce fichier contient les fonctions utilitaires pures (sans état, helpers)

// ValidateStruct permet de valider manuellement une structure
// en utilisant le moteur de Gin (et les tags `binding`).
func ValidateStruct(obj any) error {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		return v.Struct(obj)
	}
	return nubo_error.NewInternal() //errors.New("impossible de charger le validateur")
}

// CleanStr : Nettoyage anti-XSS et suppression des espaces superflus.
// NOTE : La protection contre les injections SQL est déléguée aux requêtes préparées (Postgres/database/sql).
func CleanStr(input string) string {
	cleaned := strings.TrimSpace(input)
	return html.EscapeString(cleaned)
}

// generateToken : Création JWT
func GenerateToken(userID int64, firebaseInstallationID string, expirationSeconds int) (string, error) {
	claims := jwt.MapClaims{
		"sub": fmt.Sprintf("%d", userID),
		"dev": firebaseInstallationID, // Ajout du claim personnalisé
		"exp": time.Now().Add(time.Second * time.Duration(expirationSeconds)).Unix(),
		"iat": time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", nubo_error.NewInternal() //errors.New("JWT_SECRET manquant dans les variables d'environnement")
	}
	return token.SignedString([]byte(secret))
}

// ToMap convertit une structure en map[string]any en préservant les types Go exacts (int, time.Time, etc.)
// Cela corrige les erreurs de validation "attendu struct, reçu int64".
func ToMap(in any) (map[string]any, error) {
	out := make(map[string]any)
	v := reflect.ValueOf(in)

	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil, nubo_error.NewInternal() //errors.New("ToMap: attend une struct")
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)

		// Récupération de la clé via le tag bson ou json
		tag := field.Tag.Get("bson")
		if tag == "" {
			tag = field.Tag.Get("json")
		}

		// Nettoyage des options comme "name,omitempty"
		key := strings.Split(tag, ",")[0]

		// On ignore les champs sans tag ou ignorés
		if key == "" || key == "-" {
			continue
		}

		out[key] = v.Field(i).Interface()
	}
	return out, nil
}

// toStruct convertit une map[string]any en structure (comme User)
// en respectant les tags `bson:"..."` pour que la validation du schéma fonctionne.
func ToStruct(m map[string]any, out any) error {
	// 1. On convertit la map en bytes BSON
	data, err := bson.Marshal(m)
	if err != nil {
		return err
	}

	// 2. On reconvertit les bytes en struct cible
	err = bson.Unmarshal(data, out)
	if err != nil {
		return err
	}

	return nil
}

// exists vérifie si une valeur existe dans une slice
func Exists[T comparable](slice []T, value T) bool {
	return slices.Contains(slice, value)
}

func SliceUniqueInt64(slice []int64) []int64 {
	keys := make(map[int64]bool)
	var list []int64
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func SliceUniqueStr(slice []string) []string {
	keys := make(map[string]bool)
	var list []string
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

// GetUserIDFromContext extrait de manière sécurisée l'ID utilisateur du contexte Gin.
// Elle gère les conversions de types string (JWT sub), float64 (JSON) et int64.
func GetUserIDFromContext(c *gin.Context) (int64, error) {
	val, exists := c.Get("userID")
	if !exists {
		return 0, nubo_error.NewForbidden("UNAUTHORIZED", "Utilisateur non identifié dans le contexte.", nil)
	}

	switch v := val.(type) {
	case int64:
		return v, nil
	case string:
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, nubo_error.NewBadRequest("INVALID_USER_ID", "Le format de l'ID utilisateur est invalide.", err)
		}
		return id, nil
	case float64:
		return int64(v), nil
	case int:
		return int64(v), nil
	default:
		return 0, nubo_error.NewInternal() // errors.New("type userID inconnu")
	}
}

// =========================================================================
// UTILITAIRES DE TEXTE ET MENTIONS
// =========================================================================

// Compilation globale de la regex pour des performances optimales (O(N) sans recompilation).
// On cherche le motif @{id}, ex: @{123456}
var mentionRegex = regexp.MustCompile(`@{([0-9]+)}`)

// ExtractMentions parse une chaîne de caractères et retourne une liste
// dédupliquée des IDs d'utilisateurs mentionnés.
func ExtractMentions(content string) []int64 {
	matches := mentionRegex.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	var ids []int64
	for _, match := range matches {
		if len(match) == 2 {
			if id, err := strconv.ParseInt(match[1], 10, 64); err == nil {
				ids = append(ids, id)
			}
		}
	}

	// Déduplication via l'utilitaire existant dans ce même package
	return SliceUniqueInt64(ids)
}
