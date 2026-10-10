package user_settings_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/user_settings_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/service/user_settings_service"
	"github.com/gin-gonic/gin"
)

// CheckUsernameHandler godoc
// @Summary      Vérifier la disponibilité d'un nom d'utilisateur
// @Description  Vérifie de manière performante si un nom d'utilisateur est disponible avant sa réservation.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route publique ou sécurisée selon configuration middleware.
// @Description  - Required permissions: Aucune permission spécifique.
// @Description  - Relevant middleware: RateLimiter, CORS, Recovery.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: Champ `username` dans le payload JSON.
// @Description  - Validation rules: Binding GIN vers `user_settings_models.CheckUsernameInput`.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Parsing :** Extraction du `username` via `c.ShouldBindJSON`.
// @Description  2. **Normalisation :** Suppression des espaces superflus (`strings.TrimSpace`) au niveau du service.
// @Description  3. **Vérification d'Unicité (Cascade) :** Appel de `service.IsUnique` qui sonde en cascade un filtre Cuckoo, puis le Cache L1, puis la BDD L2/L3 pour valider l'unicité du `username`.
// @Description  4. **Décision :** La disponibilité est confirmée si `IsUnique` retourne 1. S'il retourne 0 ou si la chaîne est vide, l'unicité est refusée.
// @Description  5. **Réponse HTTP :**
// @Description     - Renvoie `200 OK` si le nom est disponible.
// @Description     - Renvoie `409 Conflict` s'il est indisponible ou déjà pris.
// @Description     - *Note : Aucune réservation effective n'est effectuée. Le nom peut être pris par un tiers juste après la réponse.*
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description  - **Trigger:** Payload JSON malformé ou champ manquant. | **Execution stage:** Handler (`c.ShouldBindJSON`). | **Response:** `numan_error.PublicErrorResponse`. | **Error code:** `numan_error.CodeInvalidPayload`
// @Description
// @Description  🔴 **409 Conflict:**
// @Description  - **Trigger:** Le nom d'utilisateur normalisé est vide ou déjà enregistré par un autre compte. | **Execution stage:** Service (`service.IsUnique`). | **Response:** Body vide, code HTTP 409. | **Error code:** `409 Conflict` (HTTP Status)
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        input         body   user_settings_models.CheckUsernameInput true "Nom d'utilisateur à vérifier"
// @Success      200  {object}  map[string]interface{} "Nom d'utilisateur disponible"
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload: Trigger: JSON invalide | Execution stage: Handler | Response: PublicErrorResponse | Error code: numan_error.CodeInvalidPayload"
// @Failure      409  {object}  map[string]interface{} "Conflict: Trigger: Username déjà pris ou vide | Execution stage: Service | Response: HTTP 409 | Error code: 409 Conflict"
// @Router       /check-username [post]
func CheckUsernameHandler(c *gin.Context) {
	var input user_settings_models.CheckUsernameInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Paramètre manquant ou invalide.", err))
		return
	}

	isAvailable := user_settings_service.CheckUsernameAvailability(c.Request.Context(), input.Username)
	if isAvailable {
		c.Status(http.StatusOK)
	} else {
		c.Status(http.StatusConflict)
	}
}
