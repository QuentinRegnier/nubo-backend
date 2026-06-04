package like_handlers

import (
	"net/http"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/like_models"
	"github.com/QuentinRegnier/nubo-backend/internal/service/like_service"
	"github.com/gin-gonic/gin"

	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

// LikePostHandler godoc
// @Summary      Aimer ou ne plus aimer une publication (Asynchrone)
// @Description  Ajoute ou retire un like en mode "Fire-and-Forget" (Optimistic UI Update).
// @Description  Le système garantit l'idempotence instantanée (en RAM) et délègue la validation de la matrice de confidentialité aux workers en arrière-plan pour maintenir une latence ~1ms.
// @Description  Cette route nécessite une authentification par JWT et une signature HMAC valide.
// @Description
// @Description  **Règles de validation & Erreurs :**
// @Description
// @Description  🔴 **400 Bad Request :** Format JSON incorrect ou action non reconnue (seuls 'like' et 'unlike' sont autorisés).
// @Description  * L'ID du post dans l'URL est invalide.
// @Description
// @Description  🟠 **401 Unauthorized :** Token JWT invalide ou utilisateur non identifié.
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        id            path   int    true "ID du post"
// @Param        data          body   post_models.LikePostInput true "Action (like ou unlike)"
// @Success      200  {object}  map[string]string "message: Action prise en compte"
// @Failure      400  {object}  domain.ErrorResponse "Format JSON invalide ou ID invalide"
// @Failure      401  {object}  domain.ErrorResponse "Utilisateur non identifié"
// @Router       /post/{id}/like [post]
func LikePostHandler(c *gin.Context) {
	// 1. Sécurité
	callerUserID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Utilisateur non identifié"})
		return
	}

	// 2. Extraction du PostID
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || postID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de publication invalide"})
		return
	}

	// 3. Binding du payload et empaquetage
	var input like_models.LikePostInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "L'action doit être 'like' ou 'unlike'"})
		return
	}
	input.UserID = callerUserID
	input.PostID = postID

	// 4 & 5. Envoi au service (Ultra rapide)
	_ = like_service.TogglePostLike(c.Request.Context(), input)

	// 6. Confirmation instantanée
	c.JSON(http.StatusOK, gin.H{"message": "Action prise en compte"})
}
