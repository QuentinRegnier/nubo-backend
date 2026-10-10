package telemetry_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/telemetry_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/telemetry_service"
	"github.com/gin-gonic/gin"
)

// SyncTelemetryHandler godoc
// @Summary      Synchroniser la télémétrie client/serveur
// @Description  Exécute l'opération de synchronisation bidirectionnelle de la télémétrie et le traitement asynchrone des logs d'interaction.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée. Exige un JWT valide.
// @Description  - Required permissions: Accès contextuel lié à l'utilisateur authentifié.
// @Description  - Relevant middleware: RateLimiter, CORS, Recovery, Authentication.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: Payload JSON conforme à `telemetry_models.SyncTelemetryPayload`.
// @Description  - Validation rules: Validation GIN par tags struct.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Identification :** Extraction sécurisée du `userID` depuis le contexte (`pkg.GetUserIDFromContext`).
// @Description  2. **Parsing :** Binding du payload JSON vers la structure `telemetry_models.SyncTelemetryPayload` (`c.ShouldBindJSON`).
// @Description  3. **Contexte Serveur :** Le service `ProcessSyncTelemetry` charge le profil de télémétrie RAM L1 via `cache_service.GetTelemetryProfile`.
// @Description  4. **Résolution Vectorielle :**
// @Description     - Si client plus récent : Invalidation optionnelle du cache de feed personnalisé, mise à jour du profil RAM, et sauvegarde Write-Behind du vecteur sur l'objet `UserSettings`.
// @Description     - Si serveur plus récent : Indique au client de mettre à jour son vecteur local (`NeedUpdate = true`).
// @Description  5. **Analyse des Événements (Thundering Herd Protector) :**
// @Description     - Filtrage "Ghost Scroll" : Ignore les événements < 500ms sans clic/deep scroll.
// @Description     - Mutation L1 Synchrone : Si lecture cognitive (>1.5s ou clic), le compteur `ViewCount` du post est incrémenté dans l'Object Cache et le post est réévalué algorithmiquement.
// @Description     - Persistance Différée : Délégation des logs de télémétrie aux Workers via `redis.EnqueueDB` (Write-Behind SQL).
// @Description  6. **Réponse HTTP :** Retourne 200 OK avec directive de mise à jour, ou 202 Accepted si traité de manière purement asynchrone.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description  - **Trigger:** Format JSON invalide ou paramètres de télémétrie manquants. | **Execution stage:** Handler (`c.ShouldBindJSON`). | **Response:** `numan_error.PublicErrorResponse`. | **Error code:** `numan_error.CodeInvalidPayload`
// @Description
// @Description  🔴 **401 Unauthorized:**
// @Description  - **Trigger:** Jeton d'authentification absent, expiré ou ID utilisateur non extractible. | **Execution stage:** Handler (`pkg.GetUserIDFromContext`). | **Response:** `numan_error.PublicErrorResponse`. | **Error code:** `numan_error.CodeUnauthorized`
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description  - **Trigger:** Échec de la mise en file d'attente Redis pour le Write-Behind des événements. | **Execution stage:** Service (`ProcessSyncTelemetry`). | **Response:** `numan_error.PublicErrorResponse`. | **Error code:** `numan_error.CodeInternalError`
// @Tags         sync
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   telemetry_models.SyncTelemetryPayload true "Payload de la télémétrie (Vecteur, Tags, Événements)"
// @Success      200  {object}  telemetry_models.SyncTelemetryOutput "Synchronisation réussie avec directives de mise à jour"
// @Success      202  {object}  telemetry_models.SyncTelemetryOutput "Données télémétriques acceptées pour traitement asynchrone"
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload: Trigger: JSON invalide | Execution stage: Handler | Response: PublicErrorResponse | Error code: numan_error.CodeInvalidPayload"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Unauthorized: Trigger: Jeton absent/invalide | Execution stage: Handler | Response: PublicErrorResponse | Error code: numan_error.CodeUnauthorized"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Error: Trigger: Échec Write-Behind | Execution stage: Service | Response: PublicErrorResponse | Error code: numan_error.CodeInternalError"
// @Router       /sync/telemetry [patch]
func SyncTelemetryHandler(c *gin.Context) {
	// 1. SÉCURITÉ
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	// 2. PARSING DU PAYLOAD
	var input telemetry_models.SyncTelemetryPayload
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide.", err))
		return
	}

	// 3. APPEL DU SERVICE
	output, err := telemetry_service.ProcessSyncTelemetry(c.Request.Context(), callerID, telemetry_models.SyncTelemetryInput{Payload: input})
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	// 4. RÉPONSE DYNAMIQUE (200 OK vs 202 Accepted)
	if output.NeedUpdate {
		c.JSON(http.StatusOK, output) // Le client doit lire le body et se mettre à jour
	} else {
		c.JSON(http.StatusAccepted, output) // On a accepté et processé sa donnée en async
	}
}
