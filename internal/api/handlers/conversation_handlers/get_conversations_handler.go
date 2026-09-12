package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// GetConversationsHandler godoc
// @Summary      Récupérer des conversations spécifiques en masse
// @Description  Récupère les métadonnées fraîches d'une ou plusieurs conversations (Titre, Avatars, Non-lus) via une requête POST propre. Ignore silencieusement les IDs illégaux.
// @Tags         conversations
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        data          body   conversation_models.GetConversationsByIDInput true "Tableau des IDs de conversations"
// @Success      200  {array}  conversation_models.InboxConversationView
// @Failure      400  {object} nubo_error.PublicErrorResponse "Format JSON invalide"
// @Failure      401  {object} nubo_error.PublicErrorResponse "Utilisateur non authentifié"
// @Router       /conversations/details [post]
func GetConversationsHandler(c *gin.Context) {
	// 1. Identification de l'appelant
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 2. Récupération du joli JSON
	var input conversation_models.GetConversationsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou tableau vide.", err))
		return
	}

	// 3. Appel du service
	views, errService := conversation_service.GetConversations(c.Request.Context(), callerID, input)
	if errService != nil {
		nubo_error.RespondWithError(c, errService)
		return
	}

	// 4. Renvoi du tableau
	c.JSON(http.StatusOK, views)
}
