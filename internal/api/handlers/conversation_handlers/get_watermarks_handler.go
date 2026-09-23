package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// GetWatermarksHandler godoc
// @Summary      Récupérer les curseurs de lecture (Watermarks)
// @Description  Récupère les derniers messages lus par chaque participant d'une conversation (Lazy Loading).
// @Description  Cette route respecte l'architecture POST Payload-Only (aucun paramètre d'URL).
// @Tags         conversations
// @Accept       json
// @Produce      json
// @Param        Authorization header string true  "Bearer <votre_jwt>"
// @Param        X-Signature   header string true  "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true  "Timestamp Unix de la requête"
// @Param        data          body   conversation_models.GetWatermarksInput true "Paramètres (ID de la conversation)"
// @Success      200  {object} conversation_models.GetWatermarksOutput "Dictionnaire des watermarks"
// @Failure      400  {object} nubo_error.PublicErrorResponse "Payload invalide"
// @Failure      401  {object} nubo_error.PublicErrorResponse "Non autorisé"
// @Failure      403  {object} nubo_error.PublicErrorResponse "Non membre de la conversation"
// @Router       /conversations/watermarks/get [post]
func GetWatermarksHandler(c *gin.Context) {
	// 1. Identification de l'appelant
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 2. Parsage du JSON plat
	var input conversation_models.GetWatermarksInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou conversation_id manquant.", err))
		return
	}

	// 3. Appel du service métier
	output, errSvc := conversation_service.GetWatermarks(c.Request.Context(), callerID, input)
	if errSvc != nil {
		nubo_error.RespondWithError(c, errSvc)
		return
	}

	// 4. Renvoi du résultat
	c.JSON(http.StatusOK, output)
}
