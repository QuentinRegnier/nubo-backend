package comment_handlers

import (
	"net/http"

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
// @Failure      400  {object}  domain.ErrorResponse "Données invalides ou abus de caractères"
// @Failure      401  {object}  domain.ErrorResponse "Session expirée ou utilisateur non identifié"
// @Failure      403  {object}  domain.ErrorResponse "Violation des droits d'auteur"
// @Failure      404  {object}  domain.ErrorResponse "Commentaire introuvable"
// @Failure      500  {object}  domain.ErrorResponse "Erreur interne du serveur"
// @Router       /comment [patch]
func UpdateCommentHandler(c *gin.Context) {
	// 1. Authentification
	callerUserID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"nubo_error": "Utilisateur non identifié"})
		return
	}

	// 2. Parsing
	var input comment_models.UpdateCommentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"nubo_error": "Format JSON invalide ou champs manquants"})
		return
	}

	// 3. 🛡 BOUCLIER PHYSIQUE & NETTOYAGE : Comptage exact des caractères (runes)
	input.Content = pkg.CleanStr(input.Content)
	runeCount := len([]rune(input.Content))

	if runeCount == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"nubo_error": "Le commentaire ne peut pas être vide"})
		return
	}
	if runeCount > 2200 { // Remplace 2200 par ta limite maximale exacte
		c.JSON(http.StatusBadRequest, gin.H{"nubo_error": "Le commentaire dépasse la taille maximale autorisée"})
		return
	}

	input.UserID = callerUserID

	// 4 & 5. Appel au service métier (Cascade & Workers)
	err = comment_service.UpdateComment(c.Request.Context(), input)
	if err != nil {
		if err.Error() == "unauthorized" {
			c.JSON(http.StatusForbidden, gin.H{"nubo_error": "Vous n'êtes pas autorisé à modifier ce commentaire"})
			return
		}
		if err.Error() == "not found" {
			c.JSON(http.StatusNotFound, gin.H{"nubo_error": "Commentaire introuvable"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"nubo_error": "Erreur interne lors de la modification"})
		return
	}

	// 6. Succès
	c.JSON(http.StatusOK, gin.H{"message": "Commentaire mis à jour avec succès"})
}
