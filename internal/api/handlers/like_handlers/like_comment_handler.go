package like_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
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
// @Failure      400  {object}  nubo_error.PublicErrorResponse "Format JSON invalide ou action non reconnue"
// @Failure      401  {object}  nubo_error.PublicErrorResponse "Utilisateur non identifié"
// @Failure      404  {object}  nubo_error.PublicErrorResponse "Commentaire introuvable"
// @Router       /comment/like [post]
func LikeCommentHandler(c *gin.Context) {
	callerUserID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input like_models.LikeCommentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou action non reconnue ('like'/'unlike' attendu).", err))
		return
	}

	input.UserID = callerUserID

	err = like_service.ToggleCommentLike(c.Request.Context(), input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Action prise en compte"})
}
