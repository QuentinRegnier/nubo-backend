package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// UpdateConversationHandler godoc
// @Summary      Modifier une conversation
// @Description  Met à jour le titre et/ou les règles d'un groupe ou d'une communauté.
// @Description  Cette route nécessite des droits d'administration (Admin ou Propriétaire).
// @Tags         conversations
// @Accept       json
// @Produce      json
// @Param        Authorization header string true  "Bearer <votre_jwt>"
// @Param        X-Signature   header string true  "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true  "Timestamp Unix de la requête"
// @Param        id            path   int    true  "ID de la conversation"
// @Param        data          body   conversation_models.UpdateConversationInput true "Champs à modifier"
// @Success      200  {object} conversation_models.UpdateConversationOutput
// @Failure      400  {object} nubo_error.PublicErrorResponse "Données ou ID invalides"
// @Failure      401  {object} nubo_error.PublicErrorResponse "Session expirée ou utilisateur non identifié"
// @Failure      403  {object} nubo_error.PublicErrorResponse "Droits d'administration insuffisants"
// @Router       /conversations/{id} [patch]
func UpdateConversationHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input conversation_models.UpdateConversationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	output, err := conversation_service.UpdateConversation(c.Request.Context(), callerID, input.ConversationID, input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, output)
}
