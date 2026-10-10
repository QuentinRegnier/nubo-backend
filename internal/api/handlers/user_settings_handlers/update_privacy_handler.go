package user_settings_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/user_settings_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/user_settings_service"
	"github.com/gin-gonic/gin"
)

// UpdatePrivacyHandler godoc
// @Summary      Mettre à jour les paramètres de confidentialité
// @Description  Applique de nouveaux paramètres de confidentialité (visibilité, messages, mentions, statut en ligne, etc.).
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route requérant une authentification par jeton.
// @Description  - Required permissions: L'utilisateur ne peut cibler que ses propres paramètres.
// @Description  - Relevant middleware: RateLimiter, CORS, Recovery, Authentication.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: Modèle JSON `user_settings_models.UpdatePrivacyInput` (11 champs de confidentialité).
// @Description  - Validation rules: Validation de types et contraintes GIN.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Sécurité :** Récupération de l'identité via `pkg.GetUserIDFromContext(c)`.
// @Description  2. **Parsing :** Traduction du body JSON vers la structure `UpdatePrivacyInput` (`c.ShouldBindJSON`).
// @Description  3. **Hydratation Objet :** Requête cascade (L1/L2/L3) via `object_cache_service.GetUserSettingsCascade` pour l'objet de base.
// @Description  4. **Remplacement Intégral (PUT logique) :** Écrasement total de la branche `Privacy` du modèle par les valeurs fournies (visibilité profil, permissions de DM, tags, localisation, etc.).
// @Description  5. **Synchronisation RAM & Speed Cache :**
// @Description     - Cache L1 (Object Cache) mis à jour pour cohérence locale.
// @Description     - **Action Critique :** Appel de `cache_service.UpdateUserSpeedCachePrivacy` pour mettre à jour instantanément les attributs vitaux (permissions de discussion/groupe) dans le Speed Cache, rendant l'impact immédiat sur tout le réseau social sans invalider les autres caches profils.
// @Description  6. **Broadcast :** Distribution en temps réel des changements via WebSocket.
// @Description  7. **Persistance en File :** Soumission de la requête asynchrone (`redis.EnqueueDB`) pour écriture relationnelle en base.
// @Description  8. **Réponse HTTP :** Retourne `200 OK` avec confirmation d'horodatage.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description  - **Trigger:** Format JSON illégal ou champs requis manquants. | **Execution stage:** Handler (`c.ShouldBindJSON`). | **Response:** `numan_error.PublicErrorResponse`. | **Error code:** `numan_error.CodeInvalidPayload`
// @Description
// @Description  🔴 **401 Unauthorized:**
// @Description  - **Trigger:** Contexte utilisateur compromis ou non-authentifié. | **Execution stage:** Handler (`pkg.GetUserIDFromContext`). | **Response:** `numan_error.PublicErrorResponse`. | **Error code:** `numan_error.CodeUnauthorized`
// @Description
// @Description  🔴 **404 Not Found:**
// @Description  - **Trigger:** L'objet de réglages de l'utilisateur n'existe pas en cascade. | **Execution stage:** Service (`GetUserSettingsCascade`). | **Response:** `numan_error.PublicErrorResponse`. | **Error code:** `numan_error.CodeNotFound`
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description  - **Trigger:** Coupure réseau ou échec lors du Write-Behind vers Redis. | **Execution stage:** Service (`UpdatePrivacy`). | **Response:** `numan_error.PublicErrorResponse`. | **Error code:** `numan_error.CodeInternalError`
// @Tags         settings
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   user_settings_models.UpdatePrivacyInput true "Nouvelles préférences de confidentialité"
// @Success      200  {object}  user_settings_models.UpdatePrivacyOutput "Mise à jour de confidentialité appliquée"
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload: Trigger: JSON invalide | Execution stage: Handler | Response: PublicErrorResponse | Error code: numan_error.CodeInvalidPayload"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Unauthorized: Trigger: Jeton invalide | Execution stage: Handler | Response: PublicErrorResponse | Error code: numan_error.CodeUnauthorized"
// @Failure      404  {object}  numan_error.PublicErrorResponse "Not Found: Trigger: Settings non trouvés | Execution stage: Service | Response: PublicErrorResponse | Error code: numan_error.CodeNotFound"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Error: Trigger: Échec Write-Behind | Execution stage: Service | Response: PublicErrorResponse | Error code: numan_error.CodeInternalError"
// @Router       /settings/privacy/update [patch]
func UpdatePrivacyHandler(c *gin.Context) {
	// 1. Extraction sécurisée de l'ID utilisateur
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	// 2. Parsage du JSON plat
	var input user_settings_models.UpdatePrivacyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	// 3. Appel du service
	output, err := user_settings_service.UpdatePrivacy(c.Request.Context(), callerID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	// 4. Succès
	c.JSON(http.StatusOK, output)
}
