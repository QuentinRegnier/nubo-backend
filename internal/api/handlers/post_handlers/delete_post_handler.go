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
// @Description  Effectue une rétractation immédiate "Soft Delete" (visibilité = -1).
// @Description  Purge instantanément les caches (Object Cache LFU) et retire le post des index vectoriels de recommandation (LSH). La suppression physique en BDD est déléguée aux workers asynchrones.
// @Description  Cette route nécessite une authentification par JWT et une signature HMAC valide.
// @Description
// @Description  **Règles de validation & Erreurs :**
// @Description
// @Description  🔴 **400 Bad Request :** Format JSON incorrect ou `post_id` manquant.
// @Description  🟠 **401 Unauthorized :** Token JWT invalide ou utilisateur non identifié.
// @Description  🔴 **403 Forbidden :** Vous n'êtes pas l'auteur de cette publication.
// @Description  ⚫ **404 Not Found :** La publication n'existe pas ou a déjà été supprimée.
// @Description  ⚫ **500 Internal Server Error :** Erreur interne lors de la purge LSH/LFU ou de la mise en file d'attente asynchrone.
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        data          body   post_models.DeletePostInput true "ID du post à supprimer"
// @Success      200  {object}  map[string]string "message: Post supprimé avec succès"
// @Failure      400  {object}  domain.ErrorResponse "Format JSON invalide ou post_id manquant"
// @Failure      401  {object}  domain.ErrorResponse "Session expirée ou utilisateur non identifié"
// @Failure      403  {object}  domain.ErrorResponse "Vous n'êtes pas autorisé à supprimer ce post"
// @Failure      404  {object}  domain.ErrorResponse "Post introuvable"
// @Failure      500  {object}  domain.ErrorResponse "Erreur interne lors de la suppression"
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

	// 2.5 Sécurisation de l'input
	input.UserID = userID

	// 3. Appel au service métier
	err = post_service.DeletePost(c.Request.Context(), input)
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
