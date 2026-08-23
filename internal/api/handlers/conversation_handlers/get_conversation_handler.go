package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// GetConversationHandler godoc
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
// @Success      200  {object} conversation_models.GetInboxOutput
// @Failure      400  {object} nubo_error.ErrorResponse "Paramètres de requête invalides"
// @Failure      401  {object} nubo_error.ErrorResponse "Session expirée ou utilisateur non identifié"
// @Failure      500  {object} nubo_error.ErrorResponse "Erreur interne serveur"
// @Router       /conversations [get]
func GetConversationHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, nubo_error.ErrorResponse{Error: "Utilisateur non identifié"})
		return
	}

	var input conversation_models.GetConversationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Format JSON invalide"})
		return
	}

	if input.Limit <= 0 || input.Limit > 100 {
		input.Limit = 50
	}

	inbox, err := conversation_service.GetUserConversationPaginated(c.Request.Context(), callerID, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, nubo_error.ErrorResponse{Error: "Impossible de charger la boîte de réception"})
		return
	}
	c.JSON(http.StatusOK, inbox)
}
