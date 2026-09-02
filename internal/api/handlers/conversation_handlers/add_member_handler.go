package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// AddMemberHandler godoc
// @Summary      Ajouter des membres dans un groupe
// @Description  Ajoute de nouveaux membres à un groupe.
// @Description  Le système valide automatiquement les droits de communication de chaque cible via le SPEED Cache (L1).
// @Description  Si `add_group_permission` est faux, l'utilisateur n'est pas ajouté directement mais reçoit un lien d'invitation.
// @Tags         conversations
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        data          body   conversation_models.AddMemberInput true "ID de conversation et liste des utilisateurs cibles"
// @Success      200  {object} conversation_models.AddMemberOutput "Contient la répartition des statuts (Added, Invited, Rejected) et les tableaux 'message_ids' et 'conversation_ids' listant les invitations générées."
// @Failure      400  {object} nubo_error.PublicErrorResponse "Données ou JSON invalides"
// @Failure      401  {object} nubo_error.PublicErrorResponse "Session expirée"
// @Failure      403  {object} nubo_error.PublicErrorResponse "Droits insuffisants"
// @Router       /user-group [post]
func AddMemberHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input conversation_models.AddMemberInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	output, err := conversation_service.AddMembersToConversation(c.Request.Context(), callerID, input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, output)
}
