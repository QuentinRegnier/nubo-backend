package comment_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/gin-gonic/gin"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/comment_service"
)

// UpdateCommentHandler godoc
// @Summary      Modifier un commentaire
// @Description  Met à jour le contenu texte d'un commentaire existant.
// @Description  La persistance est gérée de manière asynchrone (Cascade L1 -> L2 -> L3 puis Workers).
// @Description  Cette route nécessite une authentification par JWT et une signature HMAC valide.
// @Description
// @Description  **Règles de validation & Erreurs :**
// @Description  🔴 **400 Bad Request :** Format JSON incorrect, ID manquant, ou échec de la validation métier (ex: commentaire vide, composé uniquement d'espaces, ou dépassant la limite physique de caractères autorisée).
// @Description  🟠 **401 Unauthorized :** Token JWT invalide, expiré ou utilisateur non identifié.
// @Description  🔴 **403 Forbidden :** Vous n'êtes pas l'auteur de ce commentaire.
// @Description  ⚫ **404 Not Found :** Le commentaire n'existe pas ou a été supprimé.
// @Description  ⚫ **500 Internal Server Error :** Erreur interne lors de la cascade de lecture ou de la mise en file d'attente Redis.
// @Tags         comments
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        data          body   comment_models.UpdateCommentInput true "Nouveau contenu du commentaire"
// @Success      200  {object}  map[string]string "message: Commentaire mis à jour avec succès"
// @Failure      400  {object}  nubo_error.PublicErrorResponse "Données invalides ou abus de caractères"
// @Failure      401  {object}  nubo_error.PublicErrorResponse "Session expirée ou utilisateur non identifié"
// @Failure      403  {object}  nubo_error.PublicErrorResponse "Violation des droits d'auteur"
// @Failure      404  {object}  nubo_error.PublicErrorResponse "Commentaire introuvable"
// @Failure      500  {object}  nubo_error.PublicErrorResponse "Erreur interne du serveur"
// @Router       /comment [patch]
func UpdateCommentHandler(c *gin.Context) {
	callerUserID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input comment_models.UpdateCommentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou champs manquants.", err))
		return
	}

	// 🛡 BOUCLIER PHYSIQUE & NETTOYAGE : Comptage exact des caractères (runes)
	input.Content = pkg.CleanStr(input.Content)
	runeCount := len([]rune(input.Content))

	if runeCount == 0 {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("EMPTY_COMMENT", "Le commentaire ne peut pas être vide.", nil))
		return
	}
	if runeCount > 2200 {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("COMMENT_TOO_LONG", "Le commentaire dépasse la taille maximale autorisée.", nil))
		return
	}

	input.UserID = callerUserID

	err = comment_service.UpdateComment(c.Request.Context(), input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Commentaire mis à jour avec succès"})
}
