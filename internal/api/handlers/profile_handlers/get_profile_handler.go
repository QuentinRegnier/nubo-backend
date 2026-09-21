package profile_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/profile_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/profile_service"
	"github.com/gin-gonic/gin"
)

// GetProfileHandler godoc
// @Summary Charger un profil complet (Super Route)
// @Description Charge l'identité, l'avatar, les relations, l'état de la messagerie, les posts récents, et les interactions (likes/saves) en une seule requête.
// @Tags profile
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer <votre_jwt>"
// @Param X-Signature header string true "Signature HMAC de la requête"
// @Param X-Timestamp header string true "Timestamp Unix de la requête"
// @Param payload body profile_models.GetProfileInput true "Paramètres de chargement du profil"
// @Success 200 {object} profile_models.GetProfileOutput
// @Failure 400 {object} nubo_error.PublicErrorResponse
// @Failure 401 {object} nubo_error.PublicErrorResponse
// @Failure 403 {object} nubo_error.PublicErrorResponse
// @Failure 404 {object} nubo_error.PublicErrorResponse
// @Router /profile/get [post]
func GetProfileHandler(c *gin.Context) {
	// 1. Identification de l'appelant
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 2. Parsing du payload JSON
	var input profile_models.GetProfileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format de la requête invalide.", err))
		return
	}

	// Application de valeurs par défaut raisonnables si non fournies
	if input.Limit == 0 {
		input.Limit = 20
	}

	// 3. Appel du Service Hub
	output, err := profile_service.GetProfile(c.Request.Context(), callerID, input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 4. Réponse
	c.JSON(http.StatusOK, output)
}
