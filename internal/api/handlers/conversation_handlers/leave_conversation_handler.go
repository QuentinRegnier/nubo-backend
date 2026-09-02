package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// LeaveConversationHandler godoc
// @Summary      Quitter ou supprimer une conversation
// @Description  Supprime une conversation de la boîte de réception. S'il s'agit d'un groupe et que l'utilisateur est propriétaire, `new_owner_id` est requis.
// @Description  Si le dernier membre quitte, la conversation entière est marquée comme supprimée.
// @Tags         conversations
// @Accept       json
// @Produce      json
// @Param        Authorization header string true  "Bearer <votre_jwt>"
// @Param        X-Signature   header string true  "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true  "Timestamp Unix de la requête"
// @Param        id            path   int    true  "ID de la conversation"
// @Param        data          body   conversation_models.LeaveConversationInput false "Optionnel: ID du nouvel administrateur"
// @Success      200  {object} map[string]string "Message de succès"
// @Failure      400  {object} nubo_error.PublicErrorResponse "Données ou ID invalides"
// @Failure      401  {object} nubo_error.PublicErrorResponse "Session expirée ou utilisateur non identifié"
// @Failure      403  {object} nubo_error.PublicErrorResponse "Erreur de droits ou transfert de propriété manquant"
// @Router       /conversations/{id} [delete]
func LeaveConversationHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input conversation_models.LeaveConversationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	if err := conversation_service.LeaveConversation(c.Request.Context(), callerID, input.ConversationID, input); err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Conversation quittée avec succès"})
}
