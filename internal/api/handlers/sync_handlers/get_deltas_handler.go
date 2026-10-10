package sync_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/sync_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/sync_service"
	"github.com/gin-gonic/gin"
)

// GetDeltasHandler godoc
// @Summary      Synchronisation granulaire des mutations (Ledger SQLite)
// @Description  Récupère les identifiants uniques des conversations ayant subi des mutations depuis un timestamp donné, permettant la synchronisation incrémentale côté client.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée via middleware (nécessite un token JWT valide).
// @Description  - Required permissions or roles: Vérification contextuelle (membre standard).
// @Description  - Relevant middleware: ExtractUserID.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `SinceMs` (timestamp) via le modèle `sync_models.GetDeltasInput`.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Extraction de l'appelant** : Récupération du `UserID` via `pkg.GetUserIDFromContext`.
// @Description  2. **Validation GIN** : Désérialisation stricte du JSON plat contenant le curseur temporel (`SinceMs`).
// @Description  3. **Appel au service métier** : L'exécution est déléguée à `sync_service.GetDeltas`.
// @Description  4. **Interrogation du Ledger (L1)** : Requête sur le cache Redis (Ledger imitant SQLite) via `cache_service.GetModifiedConversationIDs` en O(log N) pour extraire la liste des IDs altérés depuis `SinceMs`.
// @Description  5. **Normalisation de sécurité** : Garantie algorithmique de ne jamais retourner de pointeur nil (transformation en slice vide `[]int64`).
// @Description  6. **Construction de la réponse** : Renvoi de `sync_models.GetDeltasOutput`.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: `sync_models.GetDeltasOutput` (contenant le tableau `ModifiedConversationIDs`).
// @Description  - Persistence guarantees: Lecture unilatérale en RAM (Redis) sans accès persistant DB.
// @Description  - Side effects: Aucun effet de bord ni écriture.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description
// @Description  - **[INVALID_PAYLOAD] Invalid format or validation failed:**
// @Description    - Trigger: Format JSON illisible, types incorrects, ou `since_ms` manquant/hors contraintes.
// @Description    - Execution stage: Validation GIN (`ShouldBindJSON`).
// @Description    - Response: `numan_error.PublicErrorResponse` avec message "Format JSON invalide ou paramètre since_ms manquant.".
// @Description    - Error code: `INVALID_PAYLOAD` (ou constante de `numan_error`).
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description
// @Description  - **[INTERNAL_ERROR] Défaillance L1:**
// @Description    - Trigger: Redis ou le service de cache est indisponible lors de la requête Ledger.
// @Description    - Execution stage: `GetModifiedConversationIDs` (Étape 4).
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInternal`.
// @Tags         sync
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   sync_models.GetDeltasInput true "Payload de synchronisation (curseur temporel)"
// @Success      200  {object}  sync_models.GetDeltasOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /sync/deltas [post]
func GetDeltasHandler(c *gin.Context) {
	// 1. Identification de l'appelant
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	// 2. Parsage du JSON plat
	var input sync_models.GetDeltasInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou paramètre since_ms manquant.", err))
		return
	}

	// 3. Appel du service métier
	output, errSvc := sync_service.GetDeltas(c.Request.Context(), callerID, input)
	if errSvc != nil {
		numan_error.RespondWithError(c, errSvc)
		return
	}

	// 4. Renvoi du résultat
	c.JSON(http.StatusOK, output)
}
