package user_settings_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/user_settings_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/user_settings_service"
	"github.com/gin-gonic/gin"
)

// UpdateDisplayHandler godoc
// @Summary      Mettre à jour les paramètres d'affichage
// @Description  Met à jour les paramètres esthétiques et de contenu de l'utilisateur (Langue, Thème, SafeForCommute).
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée exigeant un JWT valide.
// @Description  - Required permissions: Droit de modifier ses propres paramètres.
// @Description  - Relevant middleware: RateLimiter, CORS, Recovery, Authentication.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: Payload correspondant à `user_settings_models.UpdateDisplayInput`.
// @Description  - Validation rules: Validation structurelle GIN.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Identification :** Extraction sécurisée du `userID` du contexte de la requête (`pkg.GetUserIDFromContext`).
// @Description  2. **Parsing :** Désérialisation du JSON dans `user_settings_models.UpdateDisplayInput` (`c.ShouldBindJSON`).
// @Description  3. **Récupération Cascade :** Chargement complet de l'objet `UserSettings` via la hiérarchie L1/L2/L3 (`object_cache_service.GetUserSettingsCascade`).
// @Description  4. **Mutation Intégrale :** Remplacement des propriétés `Language`, `Theme` et `SafeForCommute`, suivi de l'actualisation du timestamp de modification.
// @Description  5. **Mise à Jour L1 & Broadcast :**
// @Description     - Écrasement synchrone dans le Cache L1 (`SetUserSettings`) pour disponibilité immédiate.
// @Description     - Diffusion temps réel en WebSocket sur les sessions actives de l'utilisateur (`realtime_service.DistributeToUsers`).
// @Description  6. **Persistance Différée :** Mise en file d'attente de la mutation (`redis.EnqueueDB`) pour écriture asynchrone SQL (Write-Behind).
// @Description  7. **Réponse HTTP :** Retourne `200 OK` avec l'horodatage de la mise à jour.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description  - **Trigger:** Format JSON invalide ou paramètres manquants. | **Execution stage:** Handler (`c.ShouldBindJSON`). | **Response:** `numan_error.PublicErrorResponse`. | **Error code:** `numan_error.CodeInvalidPayload`
// @Description
// @Description  🔴 **401 Unauthorized:**
// @Description  - **Trigger:** Identité absente ou contexte altéré. | **Execution stage:** Handler (`pkg.GetUserIDFromContext`). | **Response:** `numan_error.PublicErrorResponse`. | **Error code:** `numan_error.CodeUnauthorized`
// @Description
// @Description  🔴 **404 Not Found:**
// @Description  - **Trigger:** L'objet de configuration utilisateur est inexistant en cache et base de données. | **Execution stage:** Service (`GetUserSettingsCascade`). | **Response:** `numan_error.PublicErrorResponse`. | **Error code:** `numan_error.CodeNotFound`
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description  - **Trigger:** La mise en file d'attente Redis (Write-Behind) a échoué. | **Execution stage:** Service (`UpdateDisplay`). | **Response:** `numan_error.PublicErrorResponse`. | **Error code:** `numan_error.CodeInternalError`
// @Tags         settings
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   user_settings_models.UpdateDisplayInput true "Nouvelles préférences d'affichage"
// @Success      200  {object}  user_settings_models.UpdateDisplayOutput "Mise à jour d'affichage enregistrée"
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload: Trigger: JSON invalide | Execution stage: Handler | Response: PublicErrorResponse | Error code: numan_error.CodeInvalidPayload"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Unauthorized: Trigger: Jeton invalide | Execution stage: Handler | Response: PublicErrorResponse | Error code: numan_error.CodeUnauthorized"
// @Failure      404  {object}  numan_error.PublicErrorResponse "Not Found: Trigger: Paramètres inexistants | Execution stage: Service | Response: PublicErrorResponse | Error code: numan_error.CodeNotFound"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Error: Trigger: Échec Write-Behind | Execution stage: Service | Response: PublicErrorResponse | Error code: numan_error.CodeInternalError"
// @Router       /settings/display/update [patch]
func UpdateDisplayHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input user_settings_models.UpdateDisplayInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	output, err := user_settings_service.UpdateDisplay(c.Request.Context(), callerID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, output)
}
