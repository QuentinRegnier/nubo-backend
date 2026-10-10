package sync_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/sync_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/sync_service"
	"github.com/gin-gonic/gin"
)

// SyncMessagesHandler godoc
// @Summary      Synchronisation des messages mutés (Delta Sync)
// @Description  Récupère les payloads frais et hydratés de tous les messages d'une conversation ayant subi une mutation (Création, Édition, Soft Delete) depuis un instant T.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée.
// @Description  - Relevant middleware: ExtractUserID.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `ConversationID` et `SinceMs` (le timestamp du dernier sync local) via `sync_models.SyncMessagesInput`.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Validation GIN & Identification** : Extraction du `CallerID` et binding JSON structuré.
// @Description  2. **Zero-Trust Security (Cascade)** : Vérification stricte d'appartenance à la conversation via `security_service.LeftMember`. Bloque si l'appelant n'est plus membre ou banni.
// @Description  3. **Résolution des Mutations (L1 ZSET)** : Exécution de `GetModifiedMessageIDs` (O(log N)) pour extraire les IDs ayant un score supérieur au `SinceMs`. Arrêt prématuré si 0 mutation.
// @Description  4. **Acquisition Massive (L1 -> L3)** : Lecture par lots depuis l'Object Cache (L1). Récupération explicite des "Soft Deletes". Fallback Postgres L3 pour les misses et auto-guérison silencieuse L1 asynchrone (goroutine).
// @Description  5. **Assemblage & Hydratation (DTO)** : Construction des `MessageView` avec génération HMAC (MinIO/S3) des pièces jointes, hydratation des profils d'expéditeur (Avatar Twitch ou classique), et "Fast Path L1" des compteurs de réactions et de la réaction spécifique de l'utilisateur.
// @Description  6. **Réponse HTTP** : Renvoi de `sync_models.SyncMessagesOutput` avec l'array de `MessageView`.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: Collection paginée/temporelle de messages complets (`sync_models.SyncMessagesOutput`).
// @Description  - Persistence guarantees: Lecture massive et potentielle auto-guérison du Cache L1.
// @Description  - Side effects: Génération d'URLs Média signées par S3.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description
// @Description  - **[INVALID_PAYLOAD] Invalid format:**
// @Description    - Trigger: Le payload JSON omet le `ConversationID` ou le format est erroné.
// @Description    - Execution stage: Validation GIN.
// @Description    - Response: `numan_error.PublicErrorResponse` ("Format JSON invalide ou paramètres manquants.").
// @Description    - Error code: `INVALID_PAYLOAD`.
// @Description
// @Description  🟠 **403 Forbidden:**
// @Description
// @Description  - **[FORBIDDEN] Contrôle d'accès échoué:**
// @Description    - Trigger: L'utilisateur n'appartient pas ou a été retiré de la conversation ciblée.
// @Description    - Execution stage: Étape 2 (Vérification Zero-Trust).
// @Description    - Response: `numan_error.PublicErrorResponse` issue du security_service.
// @Description    - Error code: `numan_error.CodeForbidden`.
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description
// @Description  - **[INTERNAL_ERROR] Échec L1 Index:**
// @Description    - Trigger: Impossible de récupérer la liste des IDs altérés depuis l'infrastructure Redis.
// @Description    - Execution stage: Étape 3 (Résolution des mutations `GetModifiedMessageIDs`).
// @Description    - Response: `numan_error.PublicErrorResponse` générique.
// @Description    - Error code: `numan_error.CodeInternal`.
// @Tags         sync
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   sync_models.SyncMessagesInput true "Payload avec curseur temporel et ID de conversation"
// @Success      200  {object}  sync_models.SyncMessagesOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      403  {object}  numan_error.PublicErrorResponse "Forbidden"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /sync/messages [post]
func SyncMessagesHandler(c *gin.Context) {
	// 1. Identification de l'appelant
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	// 2. Parsage du JSON plat
	var input sync_models.SyncMessagesInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	// 3. Appel du service métier
	output, errSvc := sync_service.SyncMessages(c.Request.Context(), callerID, input)
	if errSvc != nil {
		numan_error.RespondWithError(c, errSvc)
		return
	}

	// 4. Renvoi du résultat
	c.JSON(http.StatusOK, output)
}
