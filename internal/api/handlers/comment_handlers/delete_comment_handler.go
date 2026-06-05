package comment_handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/comment_service"
)

// DeleteCommentHandler godoc
// @Summary      Supprimer un commentaire
// @Description  Effectue un "Soft Delete" d'un commentaire (visibilité = -1) et décrémente instantanément le compteur du post parent en arrière-plan.
// @Description  Purge instantanément le commentaire de l'Object Cache (L1).
// @Description  Cette route nécessite une authentification par JWT et une signature HMAC valide.
// @Description
// @Description  **Règles de validation & Erreurs :**
// @Description  🔴 **400 Bad Request :** Format JSON incorrect ou `comment_id` manquant.
// @Description  🟠 **401 Unauthorized :** Token JWT invalide ou utilisateur non identifié.
// @Description  🔴 **403 Forbidden :** Vous n'êtes pas l'auteur de ce commentaire.
// @Description  ⚫ **404 Not Found :** Le commentaire n'existe pas ou a déjà été supprimé.
// @Description  ⚫ **500 Internal Server Error :** Erreur lors de la purge L1 ou de la mise en file d'attente asynchrone.
// @Tags         comments
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        data          body   comment_models.DeleteCommentInput true "ID du commentaire à supprimer"
// @Success      200  {object}  map[string]string "message: Commentaire supprimé avec succès"
// @Failure      400  {object}  domain.ErrorResponse "Format JSON invalide ou champ manquant"
// @Failure      401  {object}  domain.ErrorResponse "Session expirée ou utilisateur non identifié"
// @Failure      403  {object}  domain.ErrorResponse "Violation des droits d'auteur"
// @Failure      404  {object}  domain.ErrorResponse "Commentaire introuvable"
// @Failure      500  {object}  domain.ErrorResponse "Erreur interne du serveur"
// @Router       /comment [delete]
func DeleteCommentHandler(c *gin.Context) {
	// 1. Authentification
	callerUserID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"nubo_error": "Utilisateur non identifié"})
		return
	}

	// 2. Parsing strict
	var input comment_models.DeleteCommentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"nubo_error": "Format JSON invalide ou comment_id manquant"})
		return
	}

	// 3. Sécurisation de l'input
	input.UserID = callerUserID

	// 4. Appel au service métier (Cascade L1->L2->L3 & Envoi Asynchrone)
	err = comment_service.DeleteComment(c.Request.Context(), input)
	if err != nil {
		if err.Error() == "unauthorized" {
			c.JSON(http.StatusForbidden, gin.H{"nubo_error": "Vous n'êtes pas autorisé à supprimer ce commentaire"})
			return
		}
		if err.Error() == "not found" {
			c.JSON(http.StatusNotFound, gin.H{"nubo_error": "Commentaire introuvable ou déjà supprimé"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"nubo_error": "Erreur interne lors de la suppression"})
		return
	}

	// 5. Confirmation instantanée
	c.JSON(http.StatusOK, gin.H{"message": "Commentaire supprimé avec succès"})
}
