package message_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/message_service"
	"github.com/gin-gonic/gin"
)

// GetMessageReactionsHandler godoc
// @Summary Récupérer la liste détaillée des réactions d'un message
// @Description Récupère la liste paginée et détaillée des utilisateurs (pseudos, avatars) ayant réagi à un message spécifique.
// @Description Cette route nécessite une authentification par JWT et une signature HMAC valide.
// @Tags messages
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer <votre_jwt>"
// @Param X-Signature header string true "Signature HMAC de la requête"
// @Param X-Timestamp header string true "Timestamp Unix de la requête"
// @Param request body message_models.GetMessageReactionsInput true "Paramètres de pagination et ID du message (Body JSON pur)"
// @Success 200 {object} message_models.GetMessageReactionsOutput
// @Failure 400 {object} nubo_error.PublicErrorResponse
// @Failure 401 {object} nubo_error.PublicErrorResponse
// @Failure 403 {object} nubo_error.PublicErrorResponse
// @Failure 404 {object} nubo_error.PublicErrorResponse
// @Failure 500 {object} nubo_error.PublicErrorResponse
// @Router /messages/reactions/list [post]
func GetMessageReactionsHandler(c *gin.Context) {
	// 1. Récupération sécurisée de l'identité de l'appelant via le middleware
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 2. Binding strict du payload JSON (Zéro paramètre d'URL)
	var input message_models.GetMessageReactionsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Le format de la requête est invalide.", err))
		return
	}

	// 3. Appel du service métier pur (Logique et accès aux données)
	output, err := message_service.GetMessageReactions(c.Request.Context(), callerID, input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 4. Retour HTTP structuré
	c.JSON(http.StatusOK, output)
}
