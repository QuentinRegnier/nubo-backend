package tools

import (
	"fmt"
	"html"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
)

// Ce fichier contient les fonctions utilitaires pures (sans état, helpers)

const TIMETOKEN = time.Hour * 24

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
func GenerateToken(userID string) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(TIMETOKEN).Unix(),
		"iat": time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", fmt.Errorf("JWT_SECRET manquant")
	}

	return token.SignedString([]byte(secret))
}

// generateTokenDevice : Déclaration pour le token device
func GenerateTokenDevice(deviceInfo []string) string {
	// Logique de hashage device ici
	return "dev_token_xyz123"
}

// ToMap convertit une structure en map[string]any en préservant les types Go exacts (int, time.Time, etc.)
// Cela corrige les erreurs de validation "attendu struct, reçu int64".
func ToMap(in interface{}) (map[string]any, error) {
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

func String2Int(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return i
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

// EstNonVide vérifie si une valeur est "non vide" selon son type
func EstNonVide(v interface{}) bool {
	val := reflect.ValueOf(v)

	switch val.Kind() {

	case reflect.String:
		return val.String() != ""

	case reflect.Slice, reflect.Array:
		return val.Len() > 0

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return val.Int() != -1

	case reflect.Struct:
		for i := 0; i < val.NumField(); i++ {
			f := val.Field(i)
			if !EstNonVide(f.Interface()) {
				return false
			}
		}
		return true

	default:
		// Pour les types non gérés, on considère que non vide = valeur zéro ?
		return !val.IsZero()
	}
}
