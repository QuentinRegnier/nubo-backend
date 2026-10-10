package sync_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/sync_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/sync_service"
	"github.com/gin-gonic/gin"
)

// SyncIdentityHandler godoc
// @Summary      Delta Sync du profil et paramètres de l'utilisateur (Cold Start)
// @Description  Permet à l'application mobile d'initier un "Cold Start" en récupérant le profil public/privé et les configurations de l'utilisateur uniquement s'ils ont été altérés côté serveur.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée (Token JWT valide).
// @Description  - Relevant middleware: ExtractUserID.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `ProfileUpdatedAt` et `SettingsUpdatedAt` dans le payload `sync_models.SyncIdentityInput`.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Identification et Parsing** : Validation GIN du payload et extraction du `CallerID` (injecté dans l'input de service).
// @Description  2. **Profil L2 -> L3 (SpeedCache Bypass)** : Interrogation directe de MongoDB (L2) car le L1 cache usuel (SpeedCache) ne stocke pas les données sensibles. En cas de cache miss L2, fallback sur Postgres L3 et auto-guérison asynchrone (Write-Behind) vers Mongo.
// @Description  3. **Évaluation Delta Profil** : Comparaison du timestamp système (`UpdatedAt`) du profil avec le `ProfileUpdatedAt` du client.
// @Description  4. **Mapping Sécurisé** : Si mis à jour, transfert stricte vers `auth_models.UserProfileView` empêchant la fuite de mots de passe, et hydratation asynchrone de l'avatar avec signature HMAC (S3).
// @Description  5. **Delta Settings L1** : Récupération des paramètres en cascade L1/L2/L3 et comparaison avec `SettingsUpdatedAt`.
// @Description  6. **Réponse** : Renvoi des flags `ProfileUpdated` et `SettingsUpdated` associés aux payloads hydratés le cas échéant.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: `sync_models.SyncIdentityOutput` contenant les booléens de mise à jour et éventuellement le profil complet.
// @Description  - Persistence guarantees: Mise en file d'attente éventuelle pour l'auto-réparation du cache L2 Mongo si lecture sur L3 Postgres.
// @Description  - Side effects: Génération d'URLs Média signées, exécution de tâches de fond d'auto-guérison L3->L2.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description
// @Description  - **[INVALID_PAYLOAD] Invalid format:**
// @Description    - Trigger: Le body JSON est manquant, corrompu ou les timestamps sont de type incorrect.
// @Description    - Execution stage: Validation GIN (`ShouldBindJSON`).
// @Description    - Response: `numan_error.PublicErrorResponse` ("Format JSON invalide.").
// @Description    - Error code: `INVALID_PAYLOAD`.
// @Description
// @Description  🟠 **404 Not Found:**
// @Description
// @Description  - **[NOT_FOUND] Profil inexistant:**
// @Description    - Trigger: L'ID de l'appelant ne retourne aucun profil ni dans MongoDB (L2) ni dans PostgreSQL (L3). Cas d'anomalie d'intégrité sévère.
// @Description    - Execution stage: Exécution du Fallback L3 (Étape 2).
// @Description    - Response: `numan_error.PublicErrorResponse` ("Profil introuvable.").
// @Description    - Error code: `numan_error.CodeNotFound`.
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description
// @Description  - **[INTERNAL_ERROR] Échec L3:**
// @Description    - Trigger: Le service de base de données source de vérité (Postgres) est inaccessible après un échec Mongo.
// @Description    - Execution stage: Fallback L3.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInternal`.
// @Tags         sync
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   sync_models.SyncIdentityInput true "Timestamps d'état local du client"
// @Success      200  {object}  sync_models.SyncIdentityOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      404  {object}  numan_error.PublicErrorResponse "Not Found"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /sync/identity [post]
func SyncIdentityHandler(c *gin.Context) {
	// 1. Identification de l'appelant
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	// 2. Parsage du JSON plat
	var input sync_models.SyncIdentityInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide.", err))
		return
	}

	// 3. Appel du service métier
	output, err := sync_service.SyncIdentity(c.Request.Context(), callerID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, output)
}
