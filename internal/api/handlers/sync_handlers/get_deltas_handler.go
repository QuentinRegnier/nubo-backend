package sync_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/sync_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/sync_service"
	"github.com/gin-gonic/gin"
)

// GetDeltasHandler godoc
// @Summary      Récupérer les deltas globaux (Ledger)
// @Description  Retourne la liste des IDs de conversations ayant subi une mutation (métadonnées, membres, messages) depuis un timestamp donné.
// @Description  Respecte l'architecture POST Payload-Only.
// @Tags         sync
// @Accept       json
// @Produce      json
// @Param        Authorization header string true  "Bearer <votre_jwt>"
// @Param        X-Signature   header string true  "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true  "Timestamp Unix de la requête"
// @Param        data          body   sync_models.GetDeltasInput true "Paramètres de synchronisation (since_ms)"
// @Success      200  {object} sync_models.GetDeltasOutput "Liste des IDs de conversations modifiées"
// @Failure      400  {object} nubo_error.PublicErrorResponse "Payload invalide"
// @Failure      401  {object} nubo_error.PublicErrorResponse "Non autorisé"
// @Failure      500  {object} nubo_error.PublicErrorResponse "Erreur interne"
// @Router       /sync/deltas [post]
func GetDeltasHandler(c *gin.Context) {
	// 1. Identification de l'appelant
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 2. Parsage du JSON plat
	var input sync_models.GetDeltasInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou paramètre since_ms manquant.", err))
		return
	}

	// 3. Appel du service métier
	output, errSvc := sync_service.GetDeltas(c.Request.Context(), callerID, input)
	if errSvc != nil {
		nubo_error.RespondWithError(c, errSvc)
		return
	}

	// 4. Renvoi du résultat
	c.JSON(http.StatusOK, output)
}
