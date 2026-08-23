package message_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/message_service"
	"github.com/gin-gonic/gin"
)

// GetMessagesHandler godoc
// @Summary      Charger l'historique des messages
// @Description  Récupère les messages d'une conversation avec une pagination par offset_id et direction (top/bottom).
// @Tags         messages
// @Produce      json
// @Param        Authorization   header string true  "Bearer <votre_jwt>"
// @Param        X-Signature     header string true  "Signature HMAC de la requête"
// @Param        X-Timestamp     header string true  "Timestamp Unix de la requête"
// @Param        conversation_id query  int    true  "ID de la conversation"
// @Param        limit           query  int    false "Nombre de messages (max 100)"
// @Param        offset_id       query  int    false "L'ID du dernier message reçu"
// @Param        direction       query  string false "Direction (top ou bottom)"
// @Success      200  {object} message_models.GetMessagesOutput
// @Failure      401  {object} nubo_error.ErrorResponse "Utilisateur non identifié"
// @Router       /messages [get]
func GetMessagesHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, nubo_error.ErrorResponse{Error: "Utilisateur non identifié"})
		return
	}

	var input message_models.GetMessagesInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Format JSON invalide"})
		return
	}

	if input.Limit <= 0 || input.Limit > 100 {
		input.Limit = 50
	}

	messages, err := message_service.GetMessages(c.Request.Context(), callerID, input)
	if err != nil {
		c.JSON(http.StatusForbidden, nubo_error.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, message_models.GetMessagesOutput{Messages: messages})
}
