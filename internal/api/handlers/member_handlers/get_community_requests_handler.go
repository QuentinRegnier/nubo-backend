package member_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/member_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/member_service"
	"github.com/gin-gonic/gin"
)

// GetCommunityRequestsHandler godoc
// @Summary Lister les demandes d'adhésion (Communautés)
// @Description Récupère la liste paginée des utilisateurs en attente d'approbation (Role = -3) pour rejoindre la communauté.
// @Description Nécessite d'être Administrateur ou Propriétaire.
// @Tags conversations
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer <votre_jwt>"
// @Param X-Signature header string true "Signature HMAC de la requête"
// @Param X-Timestamp header string true "Timestamp Unix de la requête"
// @Param input body member_models.GetCommunityRequestsInput true "Paramètres de la requête"
// @Success 200 {object} member_models.GetCommunityRequestsOutput
// @Failure 400 {object} nubo_error.PublicErrorResponse
// @Failure 401 {object} nubo_error.PublicErrorResponse
// @Failure 403 {object} nubo_error.PublicErrorResponse
// @Router /conversations/communities/requests [post]
func GetCommunityRequestsHandler(c *gin.Context) {
	// 1. Récupération du CallerID via le bon package
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 2. Récupération et parsing du JSON (POST)
	var input member_models.GetCommunityRequestsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Corps de requête invalide.", err))
		return
	}

	// Valeur par défaut pour la limite si elle n'est pas fournie
	if input.Limit == 0 {
		input.Limit = 20
	}

	// 3. Appel du service métier
	output, err := member_service.GetCommunityRequests(c.Request.Context(), callerID, input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 4. Réponse
	c.JSON(http.StatusOK, output)
}
