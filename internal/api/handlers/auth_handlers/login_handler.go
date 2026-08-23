package auth_handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
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
// @Description  * `The 'data' field containing the JSON is required` : Le champ texte 'data' est manquant.
// @Description  * `Invalid JSON format in 'data': ...` : Format JSON corrompu ou mal écrit.
// @Description  * `Validation failed: ...` : Les contraintes structurelles (email valide, champs requis) ont échoué.
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
// @Accept       multipart/form-data
// @Produce      json
// @Param        data formData string true "Données JSON (auth_models.LoginInput)"
// @Success      200  {object}  auth_models.LoginResponse
// @Failure      400  {object}  domain.ErrorResponse "Données d'entrée invalides"
// @Failure      401  {object}  domain.ErrorResponse "Identifiants incorrects"
// @Failure      403  {object}  domain.ErrorResponse "Compte inaccessible (banni/désactivé)"
// @Failure      500  {object}  domain.ErrorResponse "Erreur interne du serveur"
// @Router       /login [post]
func LoginHandler(c *gin.Context) {
	var input auth_models.LoginInput

	jsonData := c.PostForm("data")
	if jsonData == "" {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "The 'data' field containing the JSON is required"})
		return
	}

	if err := json.Unmarshal([]byte(jsonData), &input); err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Invalid JSON format in 'data': " + err.Error()})
		return
	}

	if err := pkg.ValidateStruct(&input); err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Validation failed: " + err.Error()})
		return
	}

	userID, sessions, jwtToken, err := auth_service.Login(input, []string{c.ClientIP()})
	if err != nil {
		if errors.Is(err, nubo_error.ErrInvalidCredentials) || errors.Is(err, nubo_error.ErrNotFound) {
			c.JSON(http.StatusUnauthorized, nubo_error.ErrorResponse{Error: "Invalid email or password"})
			return
		}
		if errors.Is(err, nubo_error.ErrDesactivated) {
			c.JSON(http.StatusForbidden, nubo_error.ErrorResponse{Error: "Account deactivated"})
			return
		}
		if errors.Is(err, nubo_error.ErrBanned) {
			c.JSON(http.StatusForbidden, nubo_error.ErrorResponse{Error: "Account banned"})
			return
		}

		fmt.Printf("❌ ERREUR SÉCURITÉ CRITIQUE (Login): %v\n", err)
		c.JSON(http.StatusInternalServerError, nubo_error.ErrorResponse{Error: "Internal server error"})
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
