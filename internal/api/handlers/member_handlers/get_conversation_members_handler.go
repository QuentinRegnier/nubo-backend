package member_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/member_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// GetConversationMembersHandler godoc
// @Summary      Récupérer les membres de conversations spécifiques
// @Description  Récupère la liste des participants pour un lot de conversations, hydratée avec les pseudos et les avatars. Requête POST propre.
// @Tags         conversations
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        data          body   member_models.GetConversationMembersInput true "Tableau des IDs de conversations"
// @Success      200  {array}  member_models.ConversationMembersList
// @Failure      400  {object} nubo_error.PublicErrorResponse "Format JSON invalide"
// @Failure      401  {object} nubo_error.PublicErrorResponse "Utilisateur non authentifié"
// @Router       /conversations/members [post]
func GetConversationMembersHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input member_models.GetConversationMembersInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou tableau vide.", err))
		return
	}

	views, errService := conversation_service.GetConversationMembers(c.Request.Context(), callerID, input)
	if errService != nil {
		nubo_error.RespondWithError(c, errService)
		return
	}

	c.JSON(http.StatusOK, views)
}
