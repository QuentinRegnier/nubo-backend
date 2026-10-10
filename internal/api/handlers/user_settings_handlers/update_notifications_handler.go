package user_settings_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/user_settings_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/user_settings_service"
	"github.com/gin-gonic/gin"
)

// UpdateNotificationsHandler godoc
// @Summary      Mettre à jour les paramètres de notifications
// @Description  Écrase l'intégralité des préférences de notifications (Push, Email, intéractions) de l'utilisateur.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée nécessitant un jeton d'authentification valide.
// @Description  - Required permissions: Droit de modification exclusif sur son propre compte.
// @Description  - Relevant middleware: RateLimiter, CORS, Recovery, Authentication.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: Struct `user_settings_models.UpdateNotificationsInput` (9 booléens).
// @Description  - Validation rules: Binding GIN standard.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Identification :** Récupération du `userID` courant via `pkg.GetUserIDFromContext`.
// @Description  2. **Parsing :** Désérialisation stricte du JSON vers `UpdateNotificationsInput` (`c.ShouldBindJSON`).
// @Description  3. **Chargement de l'Entité :** Récupération de l'objet complet `UserSettings` via la cascade de caches (`object_cache_service.GetUserSettingsCascade`).
// @Description  4. **Écrasement :** Application des 9 propriétés booléennes (Likes, Comments, Push, Email, etc.) sur la sous-structure `Notifications` de l'objet.
// @Description  5. **Disponibilité Immédiate :**
// @Description     - Écriture asynchrone bloquante de l'objet mis à jour dans le Cache L1 (`SetUserSettings`).
// @Description     - Broadcast WebSocket pour synchronisation multi-appareils (`realtime_service.DistributeToUsers`).
// @Description  6. **Persistance Write-Behind :** Ingestion de la mutation en tâche de fond (`redis.EnqueueDB`) vers la base SQL.
// @Description  7. **Réponse HTTP :** Retourne `200 OK` avec le timestamp de dernière modification.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description  - **Trigger:** Body JSON illisible ou mal formatté. | **Execution stage:** Handler (`c.ShouldBindJSON`). | **Response:** `numan_error.PublicErrorResponse`. | **Error code:** `numan_error.CodeInvalidPayload`
// @Description
// @Description  🔴 **401 Unauthorized:**
// @Description  - **Trigger:** Authentification défaillante ou absence du contexte utilisateur. | **Execution stage:** Handler (`pkg.GetUserIDFromContext`). | **Response:** `numan_error.PublicErrorResponse`. | **Error code:** `numan_error.CodeUnauthorized`
// @Description
// @Description  🔴 **404 Not Found:**
// @Description  - **Trigger:** Impossible de trouver les configurations de l'utilisateur dans les couches de données. | **Execution stage:** Service (`GetUserSettingsCascade`). | **Response:** `numan_error.PublicErrorResponse`. | **Error code:** `numan_error.CodeNotFound`
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description  - **Trigger:** Panne du service de queue Redis pour le Write-Behind. | **Execution stage:** Service (`UpdateNotifications`). | **Response:** `numan_error.PublicErrorResponse`. | **Error code:** `numan_error.CodeInternalError`
// @Tags         settings
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   user_settings_models.UpdateNotificationsInput true "Préférences de notification booléennes"
// @Success      200  {object}  user_settings_models.UpdateNotificationsOutput "Mise à jour des notifications enregistrée"
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload: Trigger: JSON invalide | Execution stage: Handler | Response: PublicErrorResponse | Error code: numan_error.CodeInvalidPayload"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Unauthorized: Trigger: Jeton invalide | Execution stage: Handler | Response: PublicErrorResponse | Error code: numan_error.CodeUnauthorized"
// @Failure      404  {object}  numan_error.PublicErrorResponse "Not Found: Trigger: Paramètres introuvables | Execution stage: Service | Response: PublicErrorResponse | Error code: numan_error.CodeNotFound"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Error: Trigger: Échec Write-Behind | Execution stage: Service | Response: PublicErrorResponse | Error code: numan_error.CodeInternalError"
// @Router       /settings/notifications/update [patch]
func UpdateNotificationsHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input user_settings_models.UpdateNotificationsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	output, err := user_settings_service.UpdateNotifications(c.Request.Context(), callerID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, output)
}
