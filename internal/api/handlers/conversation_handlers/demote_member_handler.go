package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// DemoteMemberHandler godoc
// @Summary      Destituer un administrateur
// @Description  Rétrograde un administrateur de groupe au rang de membre standard.
// @Description  Seul le propriétaire du groupe (rôle = 2) est autorisé à effectuer cette action.
// @Tags         conversations
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        data          body   conversation_models.DemoteMemberInput true "ID du groupe et de l'administrateur à destituer"
// @Success      200  {object} map[string]string "message: Administrateur destitué avec succès"
// @Failure      400  {object} nubo_error.PublicErrorResponse "Format JSON invalide ou champs manquants"
// @Failure      401  {object} nubo_error.PublicErrorResponse "Utilisateur non identifié"
// @Failure      403  {object} nubo_error.PublicErrorResponse "Action refusée"
// @Router       /conversations/promote [delete]
func DemoteMemberHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input conversation_models.DemoteMemberInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	if err := conversation_service.DemoteMember(c.Request.Context(), callerID, input); err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Membre promu avec succès"})
}
