package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// MuteMemberHandler godoc
// @Summary Muter ou démuter un membre
// @Description Restreint le droit de parole d'un membre jusqu'à un timestamp précis. JSON uniquement.
// @Tags conversations
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer <votre_jwt>"
// @Param X-Signature header string true "Signature HMAC de la requête"
// @Param X-Timestamp header string true "Timestamp Unix de la requête"
// @Param payload body conversation_models.MuteMemberInput true "Paramètres du Mute"
// @Success 200 {object} pkg.GenericMessageResponse
// @Failure 400 {object} nubo_error.PublicErrorResponse
// @Failure 401 {object} nubo_error.PublicErrorResponse
// @Failure 403 {object} nubo_error.PublicErrorResponse
// @Router /conversations/mute [post]
func MuteMemberHandler(c *gin.Context) {
	// 1. Extraction propre du Caller via le contexte
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 2. Parsing du JSON (URL clean !)
	var input conversation_models.MuteMemberInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_JSON", "Payload invalide.", err))
		return
	}

	// 3. Appel du Service
	if err := conversation_service.MuteMember(c.Request.Context(), callerID, input); err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 4. Succès
	c.JSON(http.StatusOK, gin.H{"message": "Statut de restriction mis à jour."})
}
