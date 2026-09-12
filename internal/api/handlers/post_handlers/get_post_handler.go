package post_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/post_service"
	"github.com/gin-gonic/gin"
)

// GetPostHandler godoc
// @Summary      Récupérer un ou plusieurs posts
// @Description  Récupère une liste de posts en masse depuis leurs IDs (Forage en cascade L1 -> L2 -> L3).
// @Description  Le système filtre automatiquement les contenus selon la matrice de visibilité stricte (Public, Abonnés, Amis, Soft Delete).
// @Description  Cette route nécessite une authentification par JWT et une signature HMAC valide.
// @Description
// @Description  **Règles de validation & Erreurs :**
// @Description
// @Description    **200 OK (Succès partiel ou total) :**
// @Description  * Retourne toujours un tableau. Si un post est inaccessible (privé, supprimé, banni), l'erreur est intégrée dans l'objet de réponse du post spécifique pour ne pas bloquer le reste de la liste.
// @Description
// @Description    **400 Bad Request (Erreurs client) :**
// @Description  * Le paramètre 'ids' est manquant dans l'URL.
// @Description  * Limite dépassée : impossible de demander plus de 50 posts simultanément (Bouclier statique).
// @Description  * Aucun ID valide n'a pu être extrait.
// @Description
// @Description    **401 Unauthorized (Authentification) :**
// @Description  * Token JWT invalide, expiré ou utilisateur non identifié.
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        ids           query  string true "Liste d'IDs séparés par des virgules (ex: ?ids=123,456)"
// @Success      200  {array}   post_models.GetPostOutput "Liste des posts hydratés (avec médias et commentaires) et/ou erreurs d'accès unitaires"
// @Failure      400  {object}  nubo_error.PublicErrorResponse "Paramètre manquant ou limite de 50 IDs dépassée"
// @Failure      401  {object}  nubo_error.PublicErrorResponse "Session expirée ou utilisateur non identifié"
// @Router       /post [get]
func GetPostHandler(c *gin.Context) {
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"nubo_error": "Utilisateur non identifié"})
		return
	}

	var input post_models.GetPostInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"nubo_error": "Format JSON invalide ou post_ids manquants"})
		return
	}

	input.PostIDs = pkg.SliceUniqueInt64(input.PostIDs)

	// BOUCLIER DE BATCH (Max 50 IDs d'un coup)
	if len(input.PostIDs) > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"nubo_error": "Limite de 50 posts simultanés dépassée"})
		return
	}

	if len(input.PostIDs) == 0 {
		// ✅ CORRECTION : Assure-toi que le JSON renvoie un tableau vide typé
		c.JSON(http.StatusOK, []post_models.GetPostOutput{})
		return
	}

	input.UserID = userID

	// Appel du service hydraté (qui renvoie maintenant des GetPostOutput avec l'auteur)
	results := post_service.GetPosts(c.Request.Context(), input)

	// Le routeur HTTP sert directement la structure DTO propre
	c.JSON(http.StatusOK, results)
}
