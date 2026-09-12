package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// SuggestMemberHandler godoc
// @Summary Suggérer des membres pour une conversation
// @Description Retourne une liste de suggestions d'utilisateurs à inviter, filtrée par droits, blocages et présence.
// @Description Si la requête (query) est vide, renvoie vos amis et abonnements. Sinon, recherche globale.
// @Description Cette route nécessite une authentification par JWT et une signature HMAC valide.
// @Tags conversations
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer <votre_jwt>"
// @Param X-Signature header string true "Signature HMAC de la requête"
// @Param X-Timestamp header string true "Timestamp Unix de la requête"
// @Param payload body conversation_models.SuggestMemberInput true "Paramètres de suggestion"
// @Success 200 {object} conversation_models.SuggestMemberOutput
// @Failure 400 {object} nubo_error.PublicErrorResponse
// @Failure 401 {object} nubo_error.PublicErrorResponse
// @Failure 403 {object} nubo_error.PublicErrorResponse
// @Failure 500 {object} nubo_error.PublicErrorResponse
// @Router /conversation/member/suggest [post]
func SuggestMemberHandler(c *gin.Context) {
	// 1. Identification sécurisée de l'appelant
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 2. Parsing et validation du payload HTTP (JSON POST)
	var input conversation_models.SuggestMemberInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Corps de requête invalide.", err))
		return
	}

	// 3. Délégation stricte au service métier (DDD)
	output, err := conversation_service.SuggestMembers(c.Request.Context(), callerID, input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 4. Réponse HTTP 200 OK
	c.JSON(http.StatusOK, output)
}
