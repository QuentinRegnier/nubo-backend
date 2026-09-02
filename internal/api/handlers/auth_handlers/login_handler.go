package auth_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/service/auth_service"
	"github.com/gin-gonic/gin"
)

// Login godoc
// @Summary      Connecter un utilisateur
// @Description  Authentifie un utilisateur via email/password, synchronise les caches chauds (Session, Speed, Timeline) et renvoie le profil complet.
// @Description
// @Description  **Règles de validation & Erreurs :**
// @Description
// @Description  🔴 **400 Bad Request (Erreurs client) :**
// @Description  * `Invalid JSON format or validation failed: ...` : Format JSON corrompu ou contraintes structurelles (email valide, champs requis) non respectées.
// @Description
// @Description  🟠 **401 Unauthorized (Authentification) :**
// @Description  * `Invalid email or password` : Identifiants incorrects ou utilisateur introuvable en base de données.
// @Description
// @Description  ⛔ **403 Forbidden (Statut du compte) :**
// @Description  * `Account deactivated` : Le compte a été volontairement désactivé par l'utilisateur.
// @Description  * `Account banned` : Le compte a été banni pour non-respect des règles.
// @Description
// @Description  ⚫ **500 Internal Server Error (Serveur) :**
// @Description  * `Internal server error` : Erreur de communication BDD ou génération de jetons défaillante.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        input body auth_models.LoginInput true "Données de connexion (email et mot de passe)"
// @Success      200  {object}  auth_models.LoginResponse
// @Failure      400  {object}  nubo_error.PublicErrorResponse "Données d'entrée invalides"
// @Failure      401  {object}  nubo_error.PublicErrorResponse "Identifiants incorrects"
// @Failure      403  {object}  nubo_error.PublicErrorResponse "Compte inaccessible (banni/désactivé)"
// @Failure      500  {object}  nubo_error.PublicErrorResponse "Erreur interne du serveur"
// @Router       /login [post]
func LoginHandler(c *gin.Context) {
	var input auth_models.LoginInput

	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou contraintes non respectées.", err))
		return
	}

	userID, sessions, jwtToken, err := auth_service.Login(input, []string{c.ClientIP()})
	if err != nil {
		// Le service a déjà qualifié l'erreur (Banni, Identifiants incorrects, Désactivé)
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, auth_models.LoginResponse{
		UserID:      userID,
		MasterToken: sessions.MasterToken,
		JWT:         jwtToken,
		ExpiresAt:   sessions.ExpiresAt,
		Message:     "Login successful",
	})
}
