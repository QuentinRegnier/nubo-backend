package cuckoo

import (
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	cuckoo "github.com/seiflotfy/cuckoofilter"
)

// Registre des filtres Cuckoo
var filters = make(map[string]*cuckoo.Filter)

// buildKey crée la clé du registre (ex: "Users:username")
func buildKey(entity redis.EntityType, field string) string {
	return fmt.Sprintf("%s:%s", entity, field)
}

// RegisterFilter ajoute un nouveau filtre pour une combinaison Entité/Champ
func RegisterFilter(entity redis.EntityType, field string, capacity uint) {
	filters[buildKey(entity, field)] = cuckoo.NewFilter(capacity)
}

// MightExist exécute l'étape 1 et 2 de ton algorithme.
// Retourne (existe, a_un_filtre).
// Si a_un_filtre est false, on doit passer à l'étape 3.
// Si existe est false (et a_un_filtre true), on est sûr à 100% que c'est unique.
func MightExist(entity redis.EntityType, field string, value string) (bool, bool) {
	filter, exists := filters[buildKey(entity, field)]
	if !exists {
		return false, false // Étape 1 : Pas de filtre pour cette donnée
	}

	// Étape 2 : Recherche dans le filtre
	return filter.Lookup([]byte(value)), true
}
