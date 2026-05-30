package auth_handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/service/auth_service"
	"github.com/gin-gonic/gin"
)

// Login godoc
// @Summary      Connecter un utilisateur
// @Description  Authentifie un utilisateur via email/password et renvoie son profil complet + token.
// @Description
// @Description  **Règles & Erreurs :**
// @Description
// @Description  🔴 **400 Bad Request :**
// @Description  * `The 'data' field containing the JSON is required` : Champ 'data' manquant.
// @Description  * `Invalid JSON format in 'data'` : Le JSON envoyé est mal formé.
// @Description
// @Description  🟠 **401 Unauthorized :**
// @Description  * `Invalid email or password` : Identifiants incorrects ou utilisateur introuvable.
// @Description
// @Description  ⛔ **403 Forbidden :**
// @Description  * `Account deactivated` : Le compte a été désactivé.
// @Description  * `Account banned` : Le compte a été banni.
// @Description
// @Description  ⚫ **500 Internal Server Error :**
// @Description  * `database error` : Erreur technique interne.
// @Tags         users
// @Accept       json,multipart/form-data
// @Produce      json
// @Param        data    formData string            false "Données JSON (si multipart/form-data)"
// @Param        request body     domain.LoginInput false "Données JSON (si application/json)"
// @Success      200  {object}  domain.LoginResponse
// @Failure      400  {object}  domain.ErrorResponse "Invalid request format"
// @Failure      401  {object}  domain.ErrorResponse "Identifiants invalides"
// @Failure      403  {object}  domain.ErrorResponse "Compte bloqué/banni"
// @Failure      500  {object}  domain.ErrorResponse "Erreur serveur"
// @Failure      400  {object}  domain.ErrorResponse "Adresse IP invalide"
// @Router       /login [post_service]
func LoginHandler(c *gin.Context) {
	var input auth_models.LoginInput
	var user models.UserRequest
	var sessions models.SessionsRequest
	var err error

	// --- A. RÉCUPÉRATION DES DONNÉES ---
	// 1. Récupération via form-data (exactement comme SignUp)
	jsonData := c.PostForm("data")

	// Si on veut être souple et accepter aussi le raw JSON (optionnel mais pratique)
	if jsonData == "" && c.ContentType() == "application/json" {
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Invalid JSON format: " + err.Error()})
			return
		}
	} else {
		// Logique Form-Data (Votre demande)
		if jsonData == "" {
			c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "The 'data' field containing the JSON is required"})
			return
		}
		if err := json.Unmarshal([]byte(jsonData), &input); err != nil {
			c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Invalid JSON format in 'data': " + err.Error()})
			return
		}
	}

	IPAddress := []string{c.ClientIP()}

	// --- B. MAPPING VERS STRUCTURE INTERNE ---
	var JWT string
	user, sessions, JWT, err = auth_service.Login(input, IPAddress)

	if err == nil {
		//go StartWebsocket()

		c.JSON(http.StatusOK, auth_models.LoginResponse{
			UserID:        user.ID,
			Username:      user.Username,
			Email:         user.Email,
			EmailVerified: user.EmailVerified,
			Phone:         user.Phone,
			PhoneVerified: user.PhoneVerified,
			FirstName:     user.FirstName,
			LastName:      user.LastName,
			Birthdate:     user.Birthdate,
			Sex:           user.Sex,
			Bio:           user.Bio,
			Grade:         user.Grade,
			Location:      user.Location,
			School:        user.School,
			Work:          user.Work,
			Badges:        user.Badges,
			Desactivated:  user.Desactivated,
			Banned:        user.Banned,
			BanReason:     user.BanReason,
			BanExpiresAt:  user.BanExpiresAt,
			CreatedAt:     user.CreatedAt,
			UpdatedAt:     user.UpdatedAt,
			MasterToken:   sessions.MasterToken,
			JWT:           JWT,
			ExpiresAt:     sessions.ExpiresAt,
			Message:       "Login successful",
		})
	} else {
		// 🔍 DIAGNOSTIC PRÉCIS
		// Si le service renvoie une erreur "Identifiants invalides" ou "Introuvable"
		if errors.Is(err, domain.ErrInvalidCredentials) || errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusUnauthorized, domain.ErrorResponse{Error: "Invalid email or password"})
			return
		}
		if errors.Is(err, domain.ErrDesactivated) {
			c.JSON(http.StatusForbidden, domain.ErrorResponse{Error: "Account deactivated"})
			return
		}
		if errors.Is(err, domain.ErrBanned) {
			c.JSON(http.StatusForbidden, domain.ErrorResponse{Error: "Account banned"})
			return
		}

		// Sinon, c'est une vraie erreur technique (ex: Redis down)
		fmt.Printf("🚨 VRAIE ERREUR INTERNE : %v\n", err)
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{Error: "database error: " + err.Error()})
		return
	}
}
