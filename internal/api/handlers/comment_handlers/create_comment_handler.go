package comment_handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/comment_service"
)

// CreateCommentHandler godoc
// @Summary      Créer un commentaire
// @Description  Ajoute un commentaire texte à une publication existante.
// @Description  Le traitement est asynchrone (Fire-and-Forget) pour garantir une latence minimale au client (~1ms).
// @Description  La validation de la matrice de confidentialité (Droit de commenter, Utilisateur non banni, Amis seulement) est effectuée en arrière-plan par le pare-feu des workers. Si le commentaire est illégal, il est détruit silencieusement.
// @Description  Cette route nécessite une authentification par JWT et une signature HMAC valide.
// @Description
// @Description  **Règles de validation & Erreurs :**
// @Description
// @Description  🔴 **400 Bad Request (Erreurs client) :**
// @Description  * `Format JSON invalide...` : Le corps de la requête ne respecte pas la structure attendue ou le type des variables est erroné.
// @Description  * `post_id manquant` : L'ID de la publication cible n'a pas été fourni.
// @Description  * `Le commentaire ne peut pas être vide` : Le champ texte est manquant ou ne contient que des espaces.
// @Description  * `Validation failed` : Le contenu du commentaire dépasse la taille maximale autorisée en base de données.
// @Description
// @Description  🟠 **401 Unauthorized (Authentification) :**
// @Description  * `Utilisateur non identifié` : Le userID n'a pas pu être extrait du token JWT ou la signature HMAC est invalide.
// @Tags         comments
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        data          body   comment_models.CreateCommentInput true "Données du commentaire"
// @Success      200  {object}  map[string]string "message: Commentaire en cours de publication"
// @Failure      400  {object}  domain.ErrorResponse "Format JSON invalide, post_id manquant ou contenu vide"
// @Failure      401  {object}  domain.ErrorResponse "Session expirée ou utilisateur non identifié"
// @Router       /comment [post]
func CreateCommentHandler(c *gin.Context) {
	// 1. Sécurité : Extraction de l'ID via JWT
	callerUserID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Utilisateur non identifié"})
		return
	}

	// 2. Parsing du JSON
	var input comment_models.CreateCommentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format JSON invalide ou champs manquants"})
		return
	}

	// 3. Préparation & Nettoyage
	input.UserID = callerUserID
	input.Content = pkg.CleanStr(input.Content)

	if input.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Le commentaire ne peut pas être vide"})
		return
	}

	// 4 & 5. Envoi au Service Asynchrone
	_ = comment_service.CreateComment(c.Request.Context(), input)

	// 6. Confirmation immédiate (Latence ~1ms)
	c.JSON(http.StatusOK, gin.H{"message": "Commentaire en cours de publication"})
}
