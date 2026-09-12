package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// GetUserConversationsHandler godoc
// @Summary      Charger la boîte de réception (Inbox)
// @Description  Récupère la liste des conversations actives de l'utilisateur, triées par ordre de messages récents.
// @Description  Fournit le nombre de messages non lus (unread_count) pour l'affichage des pastilles de notification.
// @Tags         conversations
// @Produce      json
// @Param        Authorization header string true  "Bearer <votre_jwt>"
// @Param        X-Signature   header string true  "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true  "Timestamp Unix de la requête"
// @Param        offset        query  int    false "Décalage pour la pagination (défaut: 0)"
// @Param        limit         query  int    false "Nombre maximum de conversations (défaut: 50, bridé à 100)"
// @Success      200  {object} conversation_models.GetUserInboxOutput
// @Failure      400  {object} nubo_error.PublicErrorResponse "Paramètres de requête invalides"
// @Failure      401  {object} nubo_error.PublicErrorResponse "Session expirée ou utilisateur non identifié"
// @Failure      500  {object} nubo_error.PublicErrorResponse "Erreur interne serveur"
// @Router       /conversations [get]
func GetUserConversationsHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input conversation_models.GetUserConversationsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide.", err))
		return
	}

	if input.Limit <= 0 || input.Limit > 100 {
		input.Limit = 50
	}

	inbox, err := conversation_service.GetUserConversationsPaginated(c.Request.Context(), callerID, input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, inbox)
}
