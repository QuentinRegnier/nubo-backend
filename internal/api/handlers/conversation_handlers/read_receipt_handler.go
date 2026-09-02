package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// ReadReceiptHandler godoc
// @Summary      Marquer une conversation comme lue
// @Description  Remet le compteur de messages non lus (unread_count) à 0 pour l'utilisateur dans cette conversation.
// @Description  Cette route nécessite que l'utilisateur soit membre de la conversation.
// @Tags         conversations
// @Accept       json
// @Produce      json
// @Param        Authorization header string true  "Bearer <votre_jwt>"
// @Param        X-Signature   header string true  "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true  "Timestamp Unix de la requête"
// @Param        id            path   int    true  "ID de la conversation"
// @Param        data          body   conversation_models.ReadReceiptInput true "ID de la conversation"
// @Success      200  {object} map[string]string "Message de succès"
// @Failure      400  {object} nubo_error.PublicErrorResponse "Données invalides"
// @Failure      401  {object} nubo_error.PublicErrorResponse "Session expirée ou utilisateur non identifié"
// @Failure      403  {object} nubo_error.PublicErrorResponse "Accès refusé ou utilisateur banni du groupe"
// @Router       /conversations/read [post]
func ReadReceiptHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input conversation_models.ReadReceiptInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	if err := conversation_service.MarkConversationAsRead(c.Request.Context(), callerID, input.ConversationID); err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Conversation marquée comme lue"})
}
