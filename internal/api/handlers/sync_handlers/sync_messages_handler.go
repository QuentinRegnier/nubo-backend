package sync_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/sync_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/sync_service"
	"github.com/gin-gonic/gin"
)

// SyncMessagesHandler godoc
// @Summary      Combler les trous de messages (Delta Sync)
// @Description  Retourne les payloads frais des messages d'une conversation ayant subi une mutation (édition, suppression, réaction) depuis un timestamp donné.
// @Description  Respecte l'architecture POST Payload-Only.
// @Tags         sync
// @Accept       json
// @Produce      json
// @Param        Authorization header string true  "Bearer <votre_jwt>"
// @Param        X-Signature   header string true  "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true  "Timestamp Unix de la requête"
// @Param        data          body   sync_models.SyncMessagesInput true "Paramètres (ConversationID, SinceMs)"
// @Success      200  {object} sync_models.SyncMessagesOutput "Liste des messages mutés (hydratés)"
// @Failure      400  {object} nubo_error.PublicErrorResponse "Payload invalide"
// @Failure      401  {object} nubo_error.PublicErrorResponse "Non autorisé"
// @Failure      403  {object} nubo_error.PublicErrorResponse "Non membre de la conversation"
// @Failure      500  {object} nubo_error.PublicErrorResponse "Erreur interne"
// @Router       /sync/conversations/messages [post]
func SyncMessagesHandler(c *gin.Context) {
	// 1. Identification de l'appelant
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 2. Parsage du JSON plat
	var input sync_models.SyncMessagesInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	// 3. Appel du service métier
	output, errSvc := sync_service.SyncMessages(c.Request.Context(), callerID, input)
	if errSvc != nil {
		nubo_error.RespondWithError(c, errSvc)
		return
	}

	// 4. Renvoi du résultat
	c.JSON(http.StatusOK, output)
}
