package relation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/relation_service"
	"github.com/gin-gonic/gin"
)

// BlockHandler godoc
// @Summary      Bloquer un utilisateur
// @Description  Passe la relation à l'état Bloqué (état = -1). Purge mutuellement les algorithmes de recommandation.
// @Tags         Relations
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        payload body relation_models.RelationActionInput true "ID de l'utilisateur à bloquer"
// @Success      200 {object} map[string]string "Message de succès"
// @Failure      400 {object} nubo_error.ErrorResponse "JSON invalide"
// @Failure      401 {object} nubo_error.ErrorResponse "Non autorisé"
// @Failure      409 {object} nubo_error.ErrorResponse "Conflit métier"
// @Router       /relation/block [post]
func BlockHandler(c *gin.Context) {
	handleBlockAction(c, "block")
}

// UnBlockHandler godoc
// @Summary      Débloquer un utilisateur
// @Description  Supprime le blocage (état = 0). L'utilisateur devra se réabonner s'il le souhaite.
// @Tags         Relations
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        payload body relation_models.RelationActionInput true "ID de l'utilisateur à débloquer"
// @Success      200 {object} map[string]string "Message de succès"
// @Router       /relation/block [delete]
func UnBlockHandler(c *gin.Context) {
	handleBlockAction(c, "unblock")
}

func handleBlockAction(c *gin.Context, action string) {
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

	if err := relation_service.ToggleBlock(c.Request.Context(), callerID, input.TargetID, action); err != nil {
		c.JSON(http.StatusConflict, nubo_error.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Action '" + action + "' exécutée avec succès"})
}
