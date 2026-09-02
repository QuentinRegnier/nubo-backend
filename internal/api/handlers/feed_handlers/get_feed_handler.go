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
// @Failure      400  {object}  nubo_error.PublicErrorResponse "Paramètres invalides"
// @Failure      401  {object}  nubo_error.PublicErrorResponse "Utilisateur non identifié"
// @Router       /feed [get]
// @Router       /feed/force [get]
func GetFeedHandler(c *gin.Context) {
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input feed_models.GetFeedInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide.", err))
		return
	}
	input.UserID = userID

	if strings.HasSuffix(c.Request.URL.Path, "/force") {
		input.Force = true
	}

	postOutput, endIndex, activeFeed, err := feed_service.GetFeed(c.Request.Context(), input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, feed_models.GetFeedOutput{
		Status:        "Feed généré et hydraté avec succès",
		ActiveFeed:    activeFeed,
		LastSeenIndex: endIndex,
		Posts:         postOutput,
	})
}
