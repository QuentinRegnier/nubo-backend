package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// BanMemberHandler godoc
// @Summary      Bannir un membre
// @Description  Expulse et bannit un membre d'un groupe ou d'une communauté.
// @Description  L'utilisateur appelant doit avoir un rôle strictement supérieur à la cible (ex: Propriétaire > Admin > Membre).
// @Description  La cible sera retirée des listes de diffusion et la conversation disparaîtra instantanément de son Inbox.
// @Tags         conversations
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        data          body   conversation_models.BanMemberInput true "ID du groupe et de l'utilisateur à bannir"
// @Success      200  {object} conversation_models.BanMemberOutput
// @Failure      400  {object} nubo_error.PublicErrorResponse "Format JSON invalide ou paramètres manquants"
// @Failure      401  {object} nubo_error.PublicErrorResponse "Utilisateur non identifié"
// @Failure      403  {object} nubo_error.PublicErrorResponse "Droits insuffisants pour bannir ce membre"
// @Router       /group/user [delete]
func BanMemberHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input conversation_models.BanMemberInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	output, err := conversation_service.BanMember(c.Request.Context(), callerID, input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, output)
}
