package like_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/service/like_service"
	"github.com/gin-gonic/gin"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/like_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

// LikeCommentHandler godoc
// @Summary      Aimer ou ne plus aimer un commentaire
// @Description  Ajoute ou retire un like sur un commentaire.
// @Description  Met à jour instantanément le classement du commentaire (ZSET) en RAM et délègue la sauvegarde en base de données aux workers en arrière-plan.
// @Description  Cette route nécessite une authentification par JWT et une signature HMAC valide.
// @Description
// @Description  **Règles de validation & Erreurs :**
// @Description  🔴 **400 Bad Request :** Format JSON incorrect ou action non reconnue (seuls 'like' et 'unlike' sont autorisés).
// @Description  ⚫ **404 Not Found :** Le commentaire ciblé n'existe pas ou a été supprimé.
// @Description  🟠 **401 Unauthorized :** Token JWT invalide ou utilisateur non identifié.
// @Tags         comments
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        data          body   like_models.LikeCommentInput true "Action (like ou unlike) et ID du commentaire"
// @Success      200  {object}  map[string]string "message: Action prise en compte"
// @Failure      400  {object}  domain.ErrorResponse "Format JSON invalide ou action non reconnue"
// @Failure      401  {object}  domain.ErrorResponse "Utilisateur non identifié"
// @Failure      404  {object}  domain.ErrorResponse "Commentaire introuvable"
// @Router       /comment/like [post]
func LikeCommentHandler(c *gin.Context) {
	// 1. Sécurité
	callerUserID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Utilisateur non identifié"})
		return
	}

	// 2. Binding du payload
	var input like_models.LikeCommentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format JSON invalide ou action non reconnue ('like'/'unlike' attendu)"})
		return
	}

	input.UserID = callerUserID

	// 3. Appel au service hybride
	err = like_service.ToggleCommentLike(c.Request.Context(), input)
	if err != nil {
		if err.Error() == "not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Commentaire introuvable ou supprimé"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erreur interne lors du traitement du like"})
		return
	}

	// 4. Succès optimiste
	c.JSON(http.StatusOK, gin.H{"message": "Action prise en compte"})
}
