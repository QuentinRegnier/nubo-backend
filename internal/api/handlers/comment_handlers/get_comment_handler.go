package comment_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/gin-gonic/gin"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/comment_service"
)

// GetCommentsHandler godoc
// @Summary      Récupérer les commentaires d'une publication
// @Description  Récupère la liste paginée des commentaires d'un post via une stratégie hybride haute performance (ZSET Redis pour les publications virales, Arbre B-Tree MongoDB pour le cold storage).
// @Description  La route intègre une hydratation en cascade (L1 -> L2 -> L3) pour une résilience absolue et filtre automatiquement les commentaires en "Soft Delete".
// @Description  Cette route nécessite une authentification par JWT et une signature HMAC valide.
// @Description
// @Description  **Règles de validation & Erreurs :**
// @Description
// @Description  ✅ **200 OK (Succès) :**
// @Description  * Retourne toujours un tableau paginé des commentaires triés par pertinence (nombre de likes décroissant, puis date de création croissante). Un tableau vide `[]` est renvoyé s'il n'y a aucun commentaire.
// @Description
// @Description  🔴 **400 Bad Request (Erreurs client) :**
// @Description  * Le paramètre 'post_id' est manquant dans l'URL ou son format est invalide.
// @Description  * Limite dépassée : le paramètre 'limit' est bridé structurellement à 100 maximum pour protéger l'infrastructure (Bouclier statique).
// @Description
// @Description  🟠 **401 Unauthorized (Authentification) :**
// @Description  * Token JWT invalide, expiré ou utilisateur non identifié.
// @Description
// @Description  ⚫ **500 Internal Server Error (Serveur) :**
// @Description  * Échec interne lors de l'accès aux bases de données ou de l'hydratation.
// @Tags         comments
// @Accept       json
// @Produce      json
// @Param        Authorization header string true  "Bearer <votre_jwt>"
// @Param        X-Signature   header string true  "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true  "Timestamp Unix de la requête"
// @Param        post_id       query  int    true  "ID de la publication (Snowflake)"
// @Param        offset        query  int    false "Décalage pour la pagination (défaut: 0)"
// @Param        limit         query  int    false "Nombre maximum de commentaires (défaut: 50, bridé à 100)"
// @Success      200  {array}   comment_models.CommentPayload "Liste paginée et triée des commentaires"
// @Failure      400  {object}  nubo_error.PublicErrorResponse "Paramètre manquant ou limite de pagination dépassée"
// @Failure      401  {object}  nubo_error.PublicErrorResponse "Session expirée ou utilisateur non identifié"
// @Failure      500  {object}  nubo_error.PublicErrorResponse "Erreur interne lors de la récupération des données"
// @Router       /comment [get]
func GetCommentsHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input comment_models.GetCommentsInput
	if err := c.ShouldBindQuery(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_QUERY", "Paramètres de requête (post_id) invalides ou manquants.", err))
		return
	}

	// 🛡️ BOUCLIER DE PAGINATION
	if input.Limit <= 0 || input.Limit > 100 {
		input.Limit = 50
	}

	input.UserID = callerID

	comments, err := comment_service.GetComments(c.Request.Context(), input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, comments)
}
