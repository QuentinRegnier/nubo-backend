package user_settings_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/service/user_settings_service"
	"github.com/gin-gonic/gin"
)

// CheckUsernameHandler godoc
// @Summary Vérifie la disponibilité d'un nom d'utilisateur
// @Description Analyse en temps réel si un nom d'utilisateur est libre pour une nouvelle inscription.
// @Description Utilise une architecture en cascade ultra-performante :
// @Description 1. Filtre de Cuckoo en RAM (O(1)) pour certifier l'absence sans I/O base de données (100% de certitude si non trouvé).
// @Description 2. SPEED Cache (Redis L1) en cas de faux positif probabiliste du Cuckoo.
// @Description 3. Stockage à Froid (MongoDB L2) en fallback.
// @Description 4. Source de Vérité (PostgreSQL L3) en dernier recours absolu.
// @Tags Authentification
// @Produce plain
// @Param username query string true "Nom d'utilisateur à vérifier"
// @Success 200 "Le nom d'utilisateur est disponible"
// @Failure 400 "Paramètre manquant ou invalide"
// @Failure 409 "Le nom d'utilisateur est déjà pris"
// @Router /check/usernames [get]
func CheckUsernameHandler(c *gin.Context) {
	// Le handler ne fait que lire la requête HTTP (ici une query string)
	username := c.Query("username")
	if username == "" {
		c.Status(http.StatusBadRequest) // 400 Bad Request
		return
	}

	// Délégation stricte au service (aucune logique métier ou base de données ici)
	isAvailable := user_settings_service.CheckUsernameAvailability(username)

	// Réponses HTTP simples, comme demandé
	if isAvailable {
		c.Status(http.StatusOK) // 200 OK (Disponible)
	} else {
		c.Status(http.StatusConflict) // 409 Conflict (Déjà pris)
	}
}
