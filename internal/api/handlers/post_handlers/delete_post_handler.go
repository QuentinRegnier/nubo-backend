package post_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
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
// @Failure      400  {object}  nubo_error.PublicErrorResponse "Format JSON invalide ou post_id manquant"
// @Failure      401  {object}  nubo_error.PublicErrorResponse "Session expirée ou utilisateur non identifié"
// @Failure      403  {object}  nubo_error.PublicErrorResponse "Vous n'êtes pas autorisé à supprimer ce post"
// @Failure      404  {object}  nubo_error.PublicErrorResponse "Post introuvable"
// @Failure      500  {object}  nubo_error.PublicErrorResponse "Erreur interne lors de la suppression"
// @Router       /post [delete]
func DeletePostHandler(c *gin.Context) {
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input post_models.DeletePostInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou post_id manquant.", err))
		return
	}

	input.UserID = userID

	err = post_service.DeletePost(c.Request.Context(), input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Post supprimé avec succès"})
}
