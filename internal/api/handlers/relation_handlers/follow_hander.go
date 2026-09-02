package relation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/relation_service"
	"github.com/gin-gonic/gin"
)

// FollowHandler godoc
// @Summary      S'abonner à un profil
// @Description  Crée une relation d'abonnement (état = 1). Met à jour le cache L1 et la BDD.
// @Tags         Relations
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        payload body relation_models.RelationActionInput true "ID de l'utilisateur à suivre"
// @Success      200 {object} map[string]string "Message de succès"
// @Failure      400 {object} nubo_error.PublicErrorResponse "JSON invalide"
// @Failure      401 {object} nubo_error.PublicErrorResponse "Non autorisé"
// @Failure      409 {object} nubo_error.PublicErrorResponse "Action impossible (Bloqué ou auto-follow)"
// @Router       /relation/follow [post]
func FollowHandler(c *gin.Context) {
	handleFollowAction(c, "follow")
}

// UnFollowHandler godoc
// @Summary      Se désabonner d'un profil
// @Description  Supprime une relation d'abonnement (retourne à l'état = 0).
// @Tags         Relations
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        payload body relation_models.RelationActionInput true "ID de l'utilisateur à ne plus suivre"
// @Success      200 {object} map[string]string "Message de succès"
// @Router       /relation/follow [delete]
func UnFollowHandler(c *gin.Context) {
	handleFollowAction(c, "unfollow")
}

func handleFollowAction(c *gin.Context, action string) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input relation_models.RelationActionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	if err := relation_service.ToggleFollow(c.Request.Context(), callerID, input.TargetID, action); err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Action '" + action + "' exécutée avec succès"})
}
