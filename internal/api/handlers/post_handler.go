package handlers

import (
	"net/http"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/feed_service"
	"github.com/gin-gonic/gin"
)

// BatchViewInput définit la structure attendue : {"post_ids": [1, 2, 3]}
type BatchViewInput struct {
	PostIDs []int64 `json:"post_ids" binding:"required"`
}

// RegisterBatchViewsHandler godoc
// @Summary      Enregistrer des vues en lot (Batching)
// @Description  Permet au client d'envoyer un tableau d'IDs de posts vus toutes les X secondes pour soulager le réseau.
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        data body BatchViewInput true "Tableau des IDs des posts vus"
// @Success      200  {object}  domain.SuccessResponse
// @Failure      400  {object}  domain.ErrorResponse "JSON invalide ou tableau trop grand"
// @Router       /views/batch [post_service]
func RegisterBatchViewsHandler(c *gin.Context) {
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{Error: "Utilisateur non identifié"})
		return
	}

	var input BatchViewInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Format JSON invalide"})
		return
	}

	// Sécurité anti-spam : on limite la taille du lot à 100 vues maximum par requête
	// (Personne ne scrolle plus de 100 posts en 10 secondes)
	if len(input.PostIDs) > 100 {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Trop d'IDs dans le lot (max 100)"})
		return
	}

	// On boucle sur les IDs pour les envoyer dans le Buffer en RAM (Temps: 0.01ms)
	for _, postID := range input.PostIDs {
		// Nettoyage anti-doublon direct : s'assurer qu'un ID n'est pas à 0
		if postID > 0 {
			feed_service.RegisterView(userID, postID)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Vues enregistrées avec succès"})
}

// GetUserPostsHandler récupère la chronologie des posts d'un utilisateur (Profil).
// @Summary      Récupérer les posts d'un utilisateur
// @Description  Récupère les publications d'un profil spécifique via le cache_service hybride (ZSET -> Mongo -> Postgres).
// @Tags         posts
// @Param        id     path    int     true  "ID de l'utilisateur"
// @Param        offset query   int     false "Offset de pagination"
// @Param        limit  query   int     false "Nombre de posts (max 50)"
// @Router       /users/{id}/posts [get]
func GetUserPostsHandler(c *gin.Context) {
	// 1. Parsing de l'ID cible (Snowflake int64)
	targetUserID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "ID utilisateur invalide"})
		return
	}

	// 2. Pagination avec protection matérielle
	offset, _ := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 64)
	limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "20"), 10, 64)

	if limit > 50 {
		limit = 50
	}

	// 3. Appel au service d'hydratation hybride
	posts, err := cache_service.GetUserProfilePosts(c.Request.Context(), targetUserID, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{Error: "Erreur lors de la récupération des posts"})
		return
	}

	// 4. On garantit un tableau vide [] au lieu de null pour le JSON
	if posts == nil {
		posts = []models.PostRequest{}
	}

	c.JSON(http.StatusOK, posts)
}

type InteractionInput struct {
	PostID int64 `json:"post_id" binding:"required"`
}

func LikeHandler(c *gin.Context) {
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{Error: "Utilisateur non identifié"})
		return
	}

	var input InteractionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Format JSON invalide"})
		return
	}

	// C'est ici que tu appelles ta fonction qui était "Unused" !
	feed_service.RegisterLike(userID, input.PostID)

	c.JSON(http.StatusOK, gin.H{"message": "post_service liked"})
}
