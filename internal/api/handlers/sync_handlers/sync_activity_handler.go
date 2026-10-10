package sync_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/sync_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/sync_service"
	"github.com/gin-gonic/gin"
)

// SyncActivityHandler godoc
// @Summary      Synchronisation du centre d'activités (Delta Sync)
// @Description  Renvoie chirurgicalement les notifications récentes non encore lues ou mises à jour par le client, en s'appuyant sur des curseurs d'état temporels et d'identifiants.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée nécessitant un Token JWT valide.
// @Description  - Relevant middleware: ExtractUserID.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `ClientUpdatedAt` (dernier timestamp d'activité lu) et `ReadUpToID` (curseur d'ID) via le modèle `sync_models.SyncActivityInput`.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Validation GIN** : Binding JSON vers `SyncActivityInput`.
// @Description  2. **Extraction de l'appelant** : Identité extraite du contexte.
// @Description  3. **Fetch (L1/L2)** : Appel de `notification_service.GetNotifications` avec une limite stricte (sommet du ZSET asynchrone).
// @Description  4. **Évaluation du Delta (Résolution de Conflit)** : Comparaison entre le sommet serveur (`latestServerTimestamp`) et les curseurs clients (`ClientUpdatedAt`, `ReadUpToID`). Si le client est à jour, arrêt prématuré (O(1)).
// @Description  5. **Filtrage Chirurgical RAM (O(N))** : Parcours itératif de la liste décroissante et extraction des vues (`notification_models.NotificationView`) strictement supérieures aux curseurs clients.
// @Description  6. **Réponse** : Assemblage d'un `SyncActivityOutput` avec flag `NeedUpdate` et nouveau curseur serveur (`ServerUpdated`).
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: `sync_models.SyncActivityOutput`. Renvoie `NeedUpdate: false` et un array vide si aucune nouvelle activité n'est détectée.
// @Description  - Persistence guarantees: Opération exclusivement en lecture (ZSET prioritaire).
// @Description  - Side effects: Aucun. Ne modifie pas l'état "lu/non-lu" des notifications.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description
// @Description  - **[INVALID_PAYLOAD] Invalid format:**
// @Description    - Trigger: Le payload JSON ne respecte pas les types exigés ou le corps est vide.
// @Description    - Execution stage: Validation GIN (`ShouldBindJSON`).
// @Description    - Response: `numan_error.PublicErrorResponse` ("Format JSON invalide.").
// @Description    - Error code: `INVALID_PAYLOAD`.
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description
// @Description  - **[INTERNAL_ERROR] Échec L1/L2:**
// @Description    - Trigger: Impossible d'aspirer le sommet du ZSET chronologique des notifications (échec global d'infrastructure).
// @Description    - Execution stage: Fetch L1/L2 initial (Étape 3).
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInternal`.
// @Tags         sync
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   sync_models.SyncActivityInput true "Curseurs temporels et d'identification client"
// @Success      200  {object}  sync_models.SyncActivityOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /sync/activity [post]
func SyncActivityHandler(c *gin.Context) {
	// 1. Identification de l'appelant
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	// 2. Parsage du JSON plat
	var input sync_models.SyncActivityInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide.", err))
		return
	}

	// 3. Appel du service métier
	output, err := sync_service.SyncActivity(c.Request.Context(), callerID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, output)
}
