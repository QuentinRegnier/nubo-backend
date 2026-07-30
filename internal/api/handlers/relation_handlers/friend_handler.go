package relation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/relation_service"
	"github.com/gin-gonic/gin"
)

// FriendHandler godoc
// @Summary      Ajouter un ami
// @Description  Passe la relation à l'état Ami (état = 2). Si la relation n'existait pas, elle est créée.
// @Tags         Relations
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        payload body relation_models.RelationActionInput true "ID de l'utilisateur à ajouter en ami"
// @Success      200 {object} map[string]string "Message de succès"
// @Failure      400 {object} nubo_error.ErrorResponse "JSON invalide"
// @Failure      401 {object} nubo_error.ErrorResponse "Non autorisé"
// @Failure      409 {object} nubo_error.ErrorResponse "Action impossible (Bloqué)"
// @Router       /relation/friend [post]
func FriendHandler(c *gin.Context) {
	handleFriendAction(c, "friend")
}

// UnFriendHandler godoc
// @Summary      Retirer un ami
// @Description  Rétrograde un ami au statut de simple abonné (état = 1).
// @Tags         Relations
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        payload body relation_models.RelationActionInput true "ID de l'utilisateur à retirer des amis"
// @Success      200 {object} map[string]string "Message de succès"
// @Router       /relation/friend [delete]
func UnFriendHandler(c *gin.Context) {
	handleFriendAction(c, "unfriend")
}

func handleFriendAction(c *gin.Context, action string) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, nubo_error.ErrorResponse{Error: "Non autorisé"})
		return
	}

	var input relation_models.RelationActionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Invalid JSON: " + err.Error()})
		return
	}

	if err := relation_service.ToggleFriend(c.Request.Context(), callerID, input.TargetID, action); err != nil {
		c.JSON(http.StatusConflict, nubo_error.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Action '" + action + "' exécutée avec succès"})
}
