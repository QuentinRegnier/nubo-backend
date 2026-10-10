package relation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/relation_service"
	"github.com/gin-gonic/gin"
)

// GetBlockedHandler godoc
// @Summary      Récupérer la liste des utilisateurs bloqués
// @Description  Renvoie la liste des comptes que l'utilisateur appelant a bloqués (relations sortantes "outgoing").
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Token requis.
// @Description  - Required permissions or roles: Membre standard.
// @Description  - Relevant middleware: ExtractUserID.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `Limit`, `Offset` via `relation_models.GetBlockedInput`.
// @Description  - Validation rules: Binding GIN standard. Valeur par défaut pour la limite (50).
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Validation GIN** : Extraction du JSON dans `relation_models.GetBlockedInput`.
// @Description  2. **Paramètres par défaut** : Si `Limit` vaut 0, il est forcé à 50.
// @Description  3. **Appel Service** : `GetBlockedUsers` requiert la récupération des relations sortantes.
// @Description  4. **Fetch Relations** : Utilisation de la méthode générique `fetchRelationsHydrated` avec `TargetID == CallerID` et la direction "outgoing".
// @Description  5. **Hydratation** : Les profils bloqués sont résolus depuis le cache L1/L2 ou BDD avec leurs vues formatées.
// @Description  6. **Réponse** : Assemblage dans `relation_models.GetBlockedOutput`.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: `relation_models.GetBlockedOutput` contenant un tableau `Users`.
// @Description  - Persistence guarantees: O(1) depuis cache ou O(log n) depuis DB (lecture seule).
// @Description  - Side effects: Aucun.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description
// @Description  - **[INVALID_JSON] Format ou validation incorrecte:**
// @Description    - Trigger: Paramètres de pagination ou JSON mal formés.
// @Description    - Execution stage: Validation GIN `ShouldBindJSON`.
// @Description    - Response: `numan_error.PublicErrorResponse` ("Corps de requête invalide.").
// @Description    - Error code: `INVALID_JSON` (ou `numan_error.CodeInvalidPayload`).
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description
// @Description  - **[INTERNAL_ERROR] Échec lors de l'hydratation:**
// @Description    - Trigger: Base de données inaccessible ou échec d'hydratation des profils.
// @Description    - Execution stage: Appel à `fetchRelationsHydrated`.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInternal`.
// @Tags         relations
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   relation_models.GetBlockedInput true "Payload pour récupérer la liste des utilisateurs bloqués"
// @Success      200  {object}  relation_models.GetBlockedOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /block/get [post]
func GetBlockedUsersHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input relation_models.GetBlockedInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Invalid JSON ou validation échouée.", err))
		return
	}

	output, errOut := relation_service.GetBlockedUsers(c.Request.Context(), callerID, input)
	if errOut != nil {
		numan_error.RespondWithError(c, errOut)
		return
	}

	c.JSON(http.StatusOK, output)
}
