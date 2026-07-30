package user_settings_service

import (
	"strings"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
)

// CheckUsernameAvailability vérifie si un nom d'utilisateur est disponible.
// Il délègue la vérification à la cascade haute performance (Cuckoo -> L1 -> L2 -> L3).
func CheckUsernameAvailability(username string) bool {
	cleanUsername := strings.TrimSpace(username)
	if cleanUsername == "" {
		return false
	}

	// service.IsUnique renvoie 1 si unique (libre), et 0 si trouvé (pris)
	// La vérification se fait sur la collection Mongo Users, mais la fonction tape
	// d'abord le Cuckoo Filter avec la clé "username:pseudo"
	return service.IsUnique(mongo.Users, "username", cleanUsername) == 1
}
