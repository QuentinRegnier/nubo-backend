package report_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/report_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/report_service"
	"github.com/gin-gonic/gin"
)

// CreateReportHandler godoc
// @Summary      Signaler un contenu ou un utilisateur
// @Description  Permet de signaler un Post (0), Commentaire (1), Conversation (2), Message (3), ou Utilisateur (5).
// @Description  Cette route supporte la sélection multiple d'IDs (particulièrement utile pour signaler plusieurs messages d'une même conversation).
// @Description  Cette route nécessite une authentification par JWT et une signature HMAC valide.
// @Description
// @Description  **Règles de validation & Erreurs :**
// @Description
// @Description  🔴 **400 Bad Request (Erreurs client) :**
// @Description  * `Format JSON invalide` : Le corps de la requête est mal formaté ou vide.
// @Description  * `Validation failed: target_type` : Le type cible n'est pas reconnu (doit être 0, 1, 2, 3 ou 5).
// @Description  * `Validation failed: target_ids` : Le tableau d'IDs est vide ou dépasse la limite autorisée (ex: max 20 messages).
// @Description  * `Validation failed: category` : La catégorie est requise (doit correspondre à la nomenclature officielle de modération).
// @Description  * `Validation failed: reason` : La justification textuelle dépasse la limite maximale (1000 caractères).
// @Description
// @Description  🟠 **401 Unauthorized (Authentification) :**
// @Description  * `Utilisateur non identifié` : Le userID n'a pas pu être extrait du token JWT ou le token est expiré/invalide.
// @Description
// @Description  ⚫ **500 Internal Server Error (Serveur) :**
// @Description  * `Impossible de traiter le signalement...` : Échec lors de la création de l'ID Snowflake ou de l'insertion de l'événement dans la file d'attente asynchrone (Redis).
// @Tags         moderation
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        data          body   moderation_models.CreateReportInput true "Payload détaillé du signalement"
// @Success      200  {object}  map[string]string "message: Votre signalement a été pris en compte..."
// @Failure      400  {object}  domain.ErrorResponse "Données invalides : vérifiez le format, le type de cible et la catégorie."
// @Failure      401  {object}  domain.ErrorResponse "Utilisateur non identifié"
// @Failure      500  {object}  domain.ErrorResponse "Erreur interne lors de la mise en file d'attente"
// @Router       /report [post]
func CreateReportHandler(c *gin.Context) {
	// 1. Sécurité (JWT)
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"nubo_error": "Utilisateur non identifié"})
		return
	}

	// 2. Binding du JSON
	var input report_models.CreateReportInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"nubo_error": "Données invalides : vérifiez le format, le type de cible et la catégorie."})
		return
	}

	input.UserID = userID

	// 3. Appel au service asynchrone
	if err := report_service.SubmitReport(c.Request.Context(), input); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"nubo_error": "Impossible de traiter le signalement pour le moment."})
		return
	}

	// 4. Réponse de succès instantanée
	c.JSON(http.StatusOK, gin.H{"message": "Votre signalement a été pris en compte et sera étudié par nos équipes."})
}
