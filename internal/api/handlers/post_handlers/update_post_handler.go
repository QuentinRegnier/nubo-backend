package post_handlers

import (
	"fmt"
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/service/post_service"
	"github.com/gin-gonic/gin"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

// UpdatePostHandler godoc
// @Summary      Modifier une publication
// @Description  Met à jour le contenu texte, les hashtags, les mentions et la visibilité d'un post existant.
// @Description  Les médias (images) ne sont pas modifiables.
// @Description  Cette route nécessite une authentification par JWT et une signature HMAC valide.
// @Description
// @Description  **Règles de validation & Erreurs :**
// @Description  🔴 **400 Bad Request :** Format JSON incorrect ou échec de la validation métier (ex: trop de hashtags).
// @Description  🟠 **401 Unauthorized :** Token JWT invalide ou utilisateur non identifié.
// @Description  🔴 **403 Forbidden :** Vous n'êtes pas l'auteur de cette publication.
// @Description  ⚫ **404 Not Found :** La publication n'existe pas.
// @Description  ⚫ **500 Internal Server Error :** Erreur de cache ou de mise en file d'attente.
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        data          body   UpdatePostInput true "Données de mise à jour"
// @Success      200  {object}  map[string]string "message: Post mis à jour avec succès"
// @Failure      400  {object}  domain.ErrorResponse
// @Failure      401  {object}  domain.ErrorResponse
// @Failure      403  {object}  domain.ErrorResponse
// @Failure      404  {object}  domain.ErrorResponse
// @Failure      500  {object}  domain.ErrorResponse
// @Router       /post [patch]
func UpdatePostHandler(c *gin.Context) {
	// 1. Authentification via le contexte Gin
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		fmt.Printf("❌ Erreur authentification : %v\n", err)
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{Error: "Utilisateur non identifié"})
		return
	}

	// 2. Parsing du JSON strict
	var input post_models.UpdatePostInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Invalid JSON format or missing post_id"})
		return
	}

	// 3. 🛡 BOUCLIER STATIQUE : Validation O(1)
	if err := pkg.ValidateStruct(&input); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Validation failed: " + err.Error()})
		return
	}

	// 4. Nettoyage de la donnée
	input.Identifiers = pkg.SliceUniqueInt64(input.Identifiers)
	input.Hashtags = pkg.SliceUniqueStr(input.Hashtags)
	input.Content = pkg.CleanStr(input.Content)

	// 5. Appel au service métier (Contrôle d'accès L1/L2/L3 et persistance)
	err = post_service.UpdatePost(c.Request.Context(), userID, input)
	if err != nil {
		// Tri sémantique des erreurs renvoyées par le service
		if err.Error() == "unauthorized" {
			c.JSON(http.StatusForbidden, domain.ErrorResponse{Error: "Vous n'êtes pas autorisé à modifier ce post"})
			return
		}
		if err.Error() == "not found" {
			c.JSON(http.StatusNotFound, domain.ErrorResponse{Error: "Post introuvable"})
			return
		}
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{Error: "Failed to update post: " + err.Error()})
		return
	}

	// 6. Réponse HTTP 200
	c.JSON(http.StatusOK, gin.H{"message": "Post mis à jour avec succès"})
}
