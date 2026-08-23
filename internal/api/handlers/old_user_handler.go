package handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
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
	_, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, nubo_error.ErrorResponse{Error: "Utilisateur non identifié"})
		return
	}

	var input auth_models.UserSearchInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Format JSON invalide ou paramètre 'q' manquant"})
		return
	}

	if input.Limit <= 0 || input.Limit > 50 {
		input.Limit = 10
	}

	users, err := cache_service.SearchUserByPrefix(c.Request.Context(), input.Prefix, input.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, nubo_error.ErrorResponse{Error: "Erreur serveur"})
		return
	}

	if users == nil {
		users = []models.UserLiteRequest{}
	}
	c.JSON(http.StatusOK, users)
}
