package sync_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/sync_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/sync_service"
	"github.com/gin-gonic/gin"
)

// SyncInboxHandler godoc
// @Summary      Synchronisation légère de l'inbox (Delta Sync O(1))
// @Description  Vérifie instantanément (en O(1) RAM) si le client possède la dernière version de sa messagerie principale.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée.
// @Description  - Relevant middleware: ExtractUserID.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `ClientUpdatedAt` (Dernier timestamp synchronisé de l'inbox cliente) via `sync_models.SyncInboxInput`.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Validation GIN & Extraction** : Analyse du corps de requête et extraction de l'identité du `callerID`.
// @Description  2. **Lecture du Marqueur L1 (O(1))** : Exécution de `redis.InboxActivity.GetInt64` pour obtenir le timestamp absolu de la dernière activité sur l'inbox de cet utilisateur.
// @Description  3. **Guérison et initialisation** : Si aucun marqueur n'existe, génération et persistence d'un timestamp par défaut dans Redis (cas de cold start absolu ou d'éviction).
// @Description  4. **Évaluation du Delta (Coupe-circuit)** : Si le `ClientUpdatedAt` est supérieur ou égal au marqueur serveur, la fonction retourne prématurément (`NeedUpdate: false`) sans solliciter la base de données.
// @Description  5. **Hydratation à froid** : Si le client est en retard, une requête globale est ordonnée pour aspirer les 50 premières conversations via `conversation_service.GetUserConversationsPaginated`.
// @Description  6. **Réponse** : Renvoi de la structure `SyncInboxOutput` (comprenant les `Conversations` et le booléen `NeedUpdate`).
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: `sync_models.SyncInboxOutput`.
// @Description  - Persistence guarantees: Mise à jour synchrone du marqueur temporel volatile (L1) si non-existant.
// @Description  - Side effects: Aspiration complexe des conversations si dé-synchronisation avérée.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description
// @Description  - **[INVALID_PAYLOAD] Invalid format:**
// @Description    - Trigger: Le body est vide ou illisible en JSON.
// @Description    - Execution stage: Validation GIN `ShouldBindJSON`.
// @Description    - Response: `numan_error.PublicErrorResponse` ("Format JSON invalide.").
// @Description    - Error code: `INVALID_PAYLOAD`.
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description
// @Description  - **[INTERNAL_ERROR] Échec Hydratation Inbox:**
// @Description    - Trigger: Le service de conversation échoue lors de la tentative de synchronisation intégrale (BDD instable, échec de pagination).
// @Description    - Execution stage: Étape 5 (Hydratation).
// @Description    - Response: `numan_error.PublicErrorResponse` générique.
// @Description    - Error code: `numan_error.CodeInternal`.
// @Tags         sync
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   sync_models.SyncInboxInput true "Payload de synchronisation"
// @Success      200  {object}  sync_models.SyncInboxOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /sync/inbox [post]
func SyncInboxHandler(c *gin.Context) {
	// 1. Identification de l'appelant
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	// 2. Parsage du JSON plat
	var input sync_models.SyncInboxInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide.", err))
		return
	}

	// 3. Appel du service métier
	output, err := sync_service.SyncInbox(c.Request.Context(), callerID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, output)
}
