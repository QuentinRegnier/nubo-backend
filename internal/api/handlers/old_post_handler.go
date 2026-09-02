package handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/worker"
	"github.com/gin-gonic/gin"
)

// BatchViewInput définit la structure attendue : {"post_ids": [1, 2, 3]}
type BatchViewInput struct {
	PostIDs []int64 `json:"post_ids" binding:"required"`
}

// RegisterBatchViewsHandler godoc
// @Summary      Enregistrer des vues en lot (Batching)
// @Description  Permet au client d'envoyer un tableau d'IDs de posts vus toutes les X secondes pour soulager le réseau.
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        data body BatchViewInput true "Tableau des IDs des posts vus"
// @Success      200  {object}  domain.SuccessResponse
// @Failure      400  {object}  nubo_error.PublicErrorResponse "JSON invalide ou tableau trop grand"
// @Router       /views/batch [post_service]
func RegisterBatchViewsHandler(c *gin.Context) {
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input BatchViewInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide.", err))
		return
	}

	if len(input.PostIDs) > 100 {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("BATCH_TOO_LARGE", "Trop d'IDs dans le lot (max 100).", nil))
		return
	}

	for _, postID := range input.PostIDs {
		if postID > 0 {
			worker.RegisterView(userID, postID)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Vues enregistrées avec succès"})
}
