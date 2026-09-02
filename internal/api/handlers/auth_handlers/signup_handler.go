package auth_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/service/auth_service"
	"github.com/gin-gonic/gin"
)

// SignUp godoc
// @Summary      Créer un compte utilisateur
// @Description  Inscription complète. (L'upload d'avatar se fait préalablement via la route Out-of-Band /media/upload).
// @Description
// @Description  **Règles de validation & Erreurs :**
// @Description
// @Description  🔴 **400 Bad Request (Erreurs client) :**
// @Description  * `Invalid JSON format or validation failed: ...` : Ton JSON est mal écrit ou des champs obligatoires sont manquants/invalides.
// @Description  * `Invalid date format. Expected format: ddmmaaaa` : La date de naissance n'est pas bonne.
// @Description  * `Gender must be 0, 1, 2, or null` : Tu as envoyé un entier invalide pour le sexe.
// @Description  * `You must be at least 13 years old` : Restrictions d'âge.
// @Description  * `Invalid birthdate` : Date absurde (ex: plus de 120 ans).
// @Description
// @Description  🟠 **409 Conflict (Doublons) :**
// @Description  * `This username is already taken` : Le pseudo est déjà en base.
// @Description  * `This email is already taken` : L'email est déjà en base.
// @Description  * `This phone number is already taken` : Le téléphone est déjà en base.
// @Description
// @Description  ⚫ **500 Internal Server Error (Problèmes serveur) :**
// @Description  * `database nubo_error` : Postgres ou Mongo ne répondent pas.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        input body auth_models.SignUpInput true "Données d'inscription de l'utilisateur"
// @Success      200  {object}  auth_models.SignUpResponse
// @Failure      400  {object}  nubo_error.PublicErrorResponse "Données invalides (Voir liste ci-dessus)"
// @Failure      409  {object}  nubo_error.PublicErrorResponse "Conflit (Pseudo, Email ou Téléphone pris)"
// @Failure      500  {object}  nubo_error.PublicErrorResponse "Erreur Serveur"
// @Router       /signup [post]
func SignUpHandler(c *gin.Context) {
	var input auth_models.SignUpInput

	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	response, err := auth_service.CreateUser(c.Request.Context(), input, c.ClientIP())
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}
