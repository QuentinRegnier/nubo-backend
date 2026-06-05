package post_handlers

import (
	"net/http"
	"strings"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/gin-gonic/gin"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/post_service"
)

// GetUserPostsHandler godoc
// @Summary      Récupérer la chronologie d'un profil
// @Description  Récupère les publications d'un utilisateur spécifique via une cascade hybride haute performance (ZSET -> Mongo -> Postgres).
// @Description  Intègre le filtrage en temps réel via la matrice de confidentialité (Abonnés, Amis, Privé, Bloqué).
// @Description  Hydrate instantanément les signatures cryptographiques HMAC pour le chargement des images stéganographiées.
// @Description  Cette route nécessite une authentification par JWT et une signature HMAC valide.
// @Description
// @Description  **Règles de validation & Erreurs :**
// @Description
// @Description  ✅ **200 OK (Succès) :**
// @Description  * Retourne toujours un tableau paginé des posts triés par date décroissante. Un tableau vide `[]` est renvoyé si le profil est vide ou inaccessible.
// @Description
// @Description  🔴 **400 Bad Request (Erreurs client) :**
// @Description  * `user_id` manquant ou mal formaté.
// @Description
// @Description  🟠 **401 Unauthorized (Authentification) :**
// @Description  * Token JWT invalide ou Session expirée.
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        Authorization header string true  "Bearer <votre_jwt>"
// @Param        X-Signature   header string true  "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true  "Timestamp Unix de la requête"
// @Param        user_id       query  int    true  "ID de l'utilisateur ciblé"
// @Param        offset        query  int    false "Décalage pour la pagination (défaut: 0)"
// @Param        limit         query  int    false "Nombre maximum de posts (défaut: 50, bridé à 100)"
// @Success      200  {array}   post_models.GetPostOutput "Liste des publications hydratées avec médias"
// @Failure      400  {object}  domain.ErrorResponse "Paramètre manquant ou invalide"
// @Failure      401  {object}  domain.ErrorResponse "Utilisateur non identifié"
// @Failure      500  {object}  domain.ErrorResponse "Erreur interne lors de la récupération"
// @Router       /post/user [get]
func GetUserPostsHandler(c *gin.Context) {
	// 1. Sécurité
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, nubo_error.ErrorResponse{Error: "Utilisateur non identifié"})
		return
	}

	// 2. Extraction & Validation des Query Params
	var input post_models.GetUserPostsInput
	if err := c.ShouldBindQuery(&input); err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Paramètres de requête (user_id) invalides ou manquants"})
		return
	}

	// Protection structurelle de la pagination
	if input.Limit > 100 {
		input.Limit = 100
	}
	input.CallerID = callerID

	// ✅ DÉTECTION DU MODE FORCE via l'URL
	if strings.HasSuffix(c.Request.URL.Path, "/force") {
		input.Force = true
	}

	// 3. Appel au service métier hybride
	posts := post_service.GetUserPosts(c.Request.Context(), input)

	// 4. Succès
	c.JSON(http.StatusOK, posts)
}
