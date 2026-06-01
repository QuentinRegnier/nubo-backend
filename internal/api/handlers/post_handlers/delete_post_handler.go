package post_handlers

import (
	"fmt"
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/gin-gonic/gin"

	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/post_service"
)

// DeletePostHandler godoc
// @Summary      Supprimer une publication
// @Description  Effectue un "Soft Delete" (visibilité = 2) et purge instantanément les caches et index vectoriels.
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        data          body   DeletePostInput true "ID du post à supprimer"
// @Success      200  {object}  map[string]string "message: Post supprimé avec succès"
// @Failure      400  {object}  domain.ErrorResponse
// @Failure      401  {object}  domain.ErrorResponse
// @Failure      403  {object}  domain.ErrorResponse
// @Failure      404  {object}  domain.ErrorResponse
// @Failure      500  {object}  domain.ErrorResponse
// @Router       /post [delete]
func DeletePostHandler(c *gin.Context) {
	// 1. Authentification
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		fmt.Printf("❌ Erreur authentification : %v\n", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Utilisateur non identifié"})
		return
	}

	// 2. Parsing de la requête
	var input post_models.DeletePostInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format JSON invalide ou post_id manquant"})
		return
	}

	// 3. Appel au service métier (qui inclut L1->L2->L3, LSH Purge et Workers)
	err = post_service.DeletePost(c.Request.Context(), userID, input.PostID)
	if err != nil {
		if err.Error() == "unauthorized" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Vous n'êtes pas autorisé à supprimer ce post"})
			return
		}
		if err.Error() == "not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Post introuvable"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erreur lors de la suppression"})
		return
	}

	// 4. Succès
	c.JSON(http.StatusOK, gin.H{"message": "Post supprimé avec succès"})
}
