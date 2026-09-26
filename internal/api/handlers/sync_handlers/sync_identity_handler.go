package sync_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/sync_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/sync_service"
	"github.com/gin-gonic/gin"
)

// SyncIdentityHandler godoc
// @Summary      Synchroniser l'identité et la télémétrie (Delta Sync)
// @Description  Met à jour le vecteur d'intérêts et retourne le Profile et les Settings s'ils ont été modifiés côté serveur.
// @Tags         identity
// @Accept       json
// @Produce      json
// @Param        Authorization header string true  "Bearer <votre_jwt>"
// @Param        X-Signature   header string true  "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true  "Timestamp Unix de la requête"
// @Param        data body sync_models.SyncIdentityInput true "Payload de synchronisation (Vecteur, Tags, Timestamps)"
// @Success      200  {object} sync_models.SyncIdentityOutput
// @Failure      400  {object} nubo_error.PublicErrorResponse "Format JSON invalide"
// @Failure      401  {object} nubo_error.PublicErrorResponse "Non autorisé"
// @Failure      500  {object} nubo_error.PublicErrorResponse "Erreur interne"
// @Router       /sync/identity [post]
func SyncIdentityHandler(c *gin.Context) {
	var input sync_models.SyncIdentityInput

	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide.", err))
		return
	}

	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}
	input.UserID = callerID

	output, err := sync_service.SyncIdentity(c.Request.Context(), input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, output)
}
