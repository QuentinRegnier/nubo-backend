package handlers

import (
	"net/http"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache"
	"github.com/gin-gonic/gin"
)

// UserSearchHandler godoc
// @Summary      Recherche rapide d'utilisateurs (Auto-complétion)
// @Description  Recherche des utilisateurs par préfixe (insensible à la casse) en utilisant l'index lexicographique en RAM (SPEED Cache).
// @Description  Renvoie une version allégée du profil (UserLite) idéale pour l'affichage instantané dans une barre de recherche.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        q             query  string true "Le préfixe à rechercher (ex: 'quent')"
// @Param        limit         query  int    false "Nombre maximum de résultats (défaut: 10, max: 50)"
// @Success      200  {array}   domain.UserLiteRequest "Liste des profils allégés correspondants"
// @Failure      400  {object}  domain.ErrorResponse "Paramètre de recherche manquant ou invalide"
// @Failure      401  {object}  domain.ErrorResponse "Non autorisé (Token invalide ou manquant)"
// @Failure      500  {object}  domain.ErrorResponse "Erreur interne de récupération Redis"
// @Router       /search/users/quick [get]
func UserSearchHandler(c *gin.Context) {
	// 1. Identification (assurée par le middleware JWT)
	_, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{Error: "Utilisateur non identifié"})
		return
	}

	// 2. Paramètre de recherche
	prefix := c.Query("q")
	if prefix == "" {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Le paramètre de recherche 'q' est requis"})
		return
	}

	// 3. Limite (avec bornes de sécurité)
	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil || limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	// 4. Appel du service
	users, err := cache.SearchUserByPrefix(c.Request.Context(), prefix, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{Error: "Erreur lors de la recherche d'utilisateurs"})
		return
	}

	// Garantie JSON tableau vide [] plutôt que 'null'
	if users == nil {
		users = []domain.UserLiteRequest{}
	}

	c.JSON(http.StatusOK, users)
}
