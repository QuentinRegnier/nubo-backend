package member_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/member_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/member_service"
	"github.com/gin-gonic/gin"
)

// PromoteMemberHandler godoc
// @Summary      Promouvoir un membre administrateur
// @Description  Promeut un membre standard d'un groupe au rang d'administrateur.
// @Description  Seul le propriétaire du groupe (rôle = 2) est autorisé à effectuer cette action.
// @Description  La mise à jour de la mémoire RAM L1 est instantanée et asynchrone pour les bases de données.
// @Tags         conversations
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        data          body   member_models.PromoteMemberInput true "ID du groupe et de l'utilisateur à promouvoir"
// @Success      200  {object} member_models.PromoteMemberOutput
// @Failure      400  {object} nubo_error.PublicErrorResponse "Format JSON invalide ou champs manquants"
// @Failure      401  {object} nubo_error.PublicErrorResponse "Utilisateur non identifié"
// @Failure      403  {object} nubo_error.PublicErrorResponse "Seul le propriétaire peut promouvoir un membre"
// @Router       /conversations/promote [put]
func PromoteMemberHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input member_models.PromoteMemberInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	output, err := member_service.PromoteMember(c.Request.Context(), callerID, input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, output)
}
