package pkg

import (
	"fmt"
	"html"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
)

// Ce fichier contient les fonctions utilitaires pures (sans état, helpers)

// cleanStr : Nettoyage anti-XSS et SQL simple
func CleanStr(input string) string {
	cleaned := strings.TrimSpace(input)
	cleaned = html.EscapeString(cleaned)
	replacer := strings.NewReplacer(
		"DROP TABLE", "", "DELETE FROM", "", "INSERT INTO", "",
		";", "", "--", "",
	)
	return replacer.Replace(cleaned)
}

// generateToken : Création JWT
func GenerateToken(userID int64, deviceToken string, expirationSeconds int) (string, error) {
	claims := jwt.MapClaims{
		"sub": fmt.Sprintf("%d", userID),
		"dev": deviceToken, // Ajout du claim personnalisé
		"exp": time.Now().Add(time.Second * time.Duration(expirationSeconds)).Unix(),
		"iat": time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", fmt.Errorf("JWT_SECRET manquant")
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
		return nil, fmt.Errorf("ToMap: attend une struct, reçu %T", in)
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
	list := []int64{}
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
	list := []string{}
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
		return 0, fmt.Errorf("userID non trouvé dans le contexte")
	}

	switch v := val.(type) {
	case int64:
		return v, nil
	case string:
		// Cas fréquent : le 'sub' du JWT est souvent une string
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("format userID string invalide: %w", err)
		}
		return id, nil
	case float64:
		// Cas fréquent : JSON unmarshal transforme les nombres en float64
		return int64(v), nil
	case int:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("type userID inconnu: %T", v)
	}
}
