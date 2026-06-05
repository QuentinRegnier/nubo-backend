package like_handlers

import (
	"net/http"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/service/like_service"
	"github.com/gin-gonic/gin"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

// GetPostLikesHandler godoc
// @Summary      Récupérer les likes d'une publication
// @Description  Retourne une liste paginée d'IDs utilisateurs ayant aimé un post spécifique.
// @Description  Le système applique d'abord une vérification stricte de la matrice de confidentialité : si l'utilisateur n'a pas le droit de voir le post, il ne peut pas voir qui l'a aimé.
// @Description  Le requêtage utilise une cascade (L2 Mongo -> L3 Postgres) pour maximiser les performances de lecture.
// @Description  Cette route nécessite une authentification par JWT et une signature HMAC valide.
// @Description
// @Description  **Règles de validation & Erreurs :**
// @Description
// @Description  🔴 **400 Bad Request :** L'ID du post est invalide ou les paramètres de pagination (limit/offset) sont hors limites.
// @Description  🟠 **401 Unauthorized :** Token JWT invalide, expiré ou utilisateur non identifié.
// @Description  🔴 **404 Not Found :** La publication n'existe pas, a été supprimée, ou la matrice de sécurité masque son existence (ex: vous êtes bloqué par l'auteur).
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        id            path   int    true "ID du post"
// @Param        limit         query  int    false "Nombre de résultats (Défaut: 20, Max: 100)"
// @Param        offset        query  int    false "Décalage pour la pagination (Défaut: 0)"
// @Success      200  {object}  post_models.GetPostLikesOutput "Liste des identifiants des utilisateurs ayant liké"
// @Failure      400  {object}  domain.ErrorResponse "Paramètres de requête invalides"
// @Failure      401  {object}  domain.ErrorResponse "Utilisateur non identifié"
// @Failure      404  {object}  domain.ErrorResponse "Post introuvable ou inaccessible"
// @Failure      500  {object}  domain.ErrorResponse "Erreur interne lors de la récupération des likes"
// @Router       /post/{id}/likes [get]
func GetPostLikesHandler(c *gin.Context) {
	// 1. Authentification
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"nubo_error": "Utilisateur non identifié"})
		return
	}

	// 2. Extraction du PostID depuis l'URL
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || postID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"nubo_error": "ID de publication invalide"})
		return
	}

	// 3. Extraction de la pagination avec valeurs par défaut
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if offset < 0 {
		offset = 0
	}

	// 4. Empaquetage de l'input strict
	input := post_models.GetPostLikesInput{
		CallerID: callerID,
		PostID:   postID,
		Limit:    limit,
		Offset:   offset,
	}

	// 5. Appel au service métier (qui inclut la vérification des droits L1->L2->L3)
	output, err := like_service.GetPostLikes(c.Request.Context(), input)
	if err != nil {
		if err.Error() == "not found" || err.Error() == "forbidden" || err.Error() == "banned" {
			// On maintient le mode furtif
			c.JSON(http.StatusNotFound, gin.H{"nubo_error": "Post introuvable ou inaccessible"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"nubo_error": "Erreur interne lors de la récupération"})
		return
	}

	// 6. Succès
	c.JSON(http.StatusOK, output)
}
