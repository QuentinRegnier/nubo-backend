package handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
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
		c.JSON(http.StatusUnauthorized, nubo_error.ErrorResponse{Error: "Utilisateur non identifié"})
		return
	}

	var input BatchViewInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Format JSON invalide"})
		return
	}

	// Sécurité anti-spam : on limite la taille du lot à 100 vues maximum par requête
	// (Personne ne scrolle plus de 100 posts en 10 secondes)
	if len(input.PostIDs) > 100 {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Trop d'IDs dans le lot (max 100)"})
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
