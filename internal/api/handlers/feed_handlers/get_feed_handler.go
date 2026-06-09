package feed_handlers

import (
	"net/http"
	"strings"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/gin-gonic/gin"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/feed_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/feed_service"
)

// GetFeedHandler godoc
// @Summary      Récupérer le distributeur principal (Feed)
// @Description  Sert la file d'actualité de l'utilisateur. Gère la rotation circulaire (A->B->C) en cas de pull-to-refresh et l'hydratation L1/L2/L3.
// @Tags         feed
// @Accept       json
// @Produce      json
// @Param        last_seen_index query int false "Index du dernier post vu (pour le scroll continu)"
// @Success      200  {object}  feed_models.GetFeedOutput
// @Failure      400  {object}  domain.ErrorResponse "Paramètres invalides"
// @Failure      401  {object}  domain.ErrorResponse "Utilisateur non identifié"
// @Router       /feed [get]
// @Router       /feed/force [get]
func GetFeedHandler(c *gin.Context) {
	// 1. Sécurité
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, nubo_error.ErrorResponse{Error: "Utilisateur non identifié"})
		return
	}

	// 2. Extraction des paramètres
	var input feed_models.GetFeedInput
	if err := c.ShouldBindQuery(&input); err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Paramètres de pagination invalides"})
		return
	}
	input.UserID = userID

	// Détection du suffixe /force (Pull-to-refresh)
	if strings.HasSuffix(c.Request.URL.Path, "/force") {
		input.Force = true
	}

	// 3. Délégation complète au service
	postOutput, endIndex, activeFeed, err := feed_service.GetFeed(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, nubo_error.ErrorResponse{Error: "Erreur interne lors de la génération du feed"})
		return
	}

	// 4. RETOUR AU CLIENT
	feedOutput := feed_models.GetFeedOutput{
		Status:        "Feed généré et hydraté avec succès",
		ActiveFeed:    activeFeed,
		LastSeenIndex: endIndex, // Le client nous renverra cet index au prochain appel
		Posts:         postOutput,
	}

	c.JSON(http.StatusOK, feedOutput)
}
