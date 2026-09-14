package user_settings_service

import (
	"context"
	"strings"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
)

// CheckUsernameAvailability vérifie si un nom d'utilisateur est disponible.
// Il délègue la vérification à la cascade haute performance (Cuckoo -> L1 -> L2 -> L3).
func CheckUsernameAvailability(ctx context.Context, username string) bool {
	cleanUsername := strings.TrimSpace(username)
	if cleanUsername == "" {
		return false
	}

	// service.IsUnique renvoie 1 si unique (libre), et 0 si trouvé (pris)
	// On passe bien le ctx et la bonne constante d'entité redis.EntityUser
	return service.IsUnique(ctx, redis.EntityUser, "username", cleanUsername) == 1
}
