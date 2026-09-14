package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// CreateCommunityHandler godoc
// @Summary Créer une communauté publique
// @Description Crée une communauté publique (Type 3). Réservé aux Collaborateurs (max 1), Modérateurs et Admins. Les modérateurs peuvent céder la propriété via `owner_id`.
// @Tags conversations
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer <votre_jwt>"
// @Param X-Signature header string true "Signature HMAC de la requête"
// @Param X-Timestamp header string true "Timestamp Unix de la requête"
// @Param request body conversation_models.CreateCommunityInput true "Données de la communauté"
// @Success 201 {object} conversation_models.CreateCommunityOutput
// @Failure 400 {object} nubo_error.PublicErrorResponse
// @Failure 401 {object} nubo_error.PublicErrorResponse
// @Failure 403 {object} nubo_error.PublicErrorResponse
// @Failure 500 {object} nubo_error.PublicErrorResponse
// @Router /communities [post]
func CreateCommunityHandler(c *gin.Context) {
	// 1. Extraction sécurisée de l'ID utilisateur
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 2. Parsing du Body
	var input conversation_models.CreateCommunityInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Le format des données est invalide.", err))
		return
	}

	// 3. Appel du service métier pur
	output, err := conversation_service.CreateCommunity(c.Request.Context(), callerID, input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 4. Réponse
	c.JSON(http.StatusCreated, output)
}
