package search_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/search_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/search_service"
	"github.com/gin-gonic/gin"
)

// AutocompleteTextHandler godoc
// @Summary Autocomplétion globale (Barre de recherche)
// @Description Recherche ultra-rapide (O(log(N)) en RAM) d'utilisateurs et de communautés à partir d'un préfixe.
// @Description Cette route nécessite une authentification par JWT et une signature HMAC valide.
// @Tags search
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer <votre_jwt>"
// @Param X-Signature header string true "Signature HMAC de la requête"
// @Param X-Timestamp header string true "Timestamp Unix de la requête"
// @Param payload body search_models.AutocompleteTextInput true "Paramètres de recherche"
// @Success 200 {object} search_models.AutocompleteTextOutput
// @Failure 400 {object} nubo_error.PublicErrorResponse
// @Failure 401 {object} nubo_error.PublicErrorResponse
// @Failure 500 {object} nubo_error.PublicErrorResponse
// @Router /search/autocomplete/text [post]
func AutocompleteTextHandler(c *gin.Context) {
	// 1. Identification sécurisée de l'appelant
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 2. Parsing et validation du payload HTTP (JSON POST)
	var input search_models.AutocompleteTextInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Corps de requête invalide.", err))
		return
	}

	// 3. Délégation stricte au service métier (DDD)
	output, err := search_service.AutocompleteText(c.Request.Context(), callerID, input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 4. Réponse HTTP 200 OK
	c.JSON(http.StatusOK, output)
}
