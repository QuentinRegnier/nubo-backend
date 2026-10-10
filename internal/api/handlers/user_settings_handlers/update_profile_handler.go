package user_settings_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/user_settings_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/user_settings_service"
	"github.com/gin-gonic/gin"
)

// UpdateProfileHandler godoc
// @Summary      Mettre à jour le profil
// @Description  Modifie les informations personnelles du profil de l'utilisateur authentifié (username, email, biographie, photo de profil, etc.).
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée. Requiert un JWT valide et une signature HMAC.
// @Description  - Required permissions or roles: Le JWT doit correspondre à l'utilisateur ciblé (`userID` extrait du contexte).
// @Description  - Relevant middleware: JWTMiddleware, HMACMiddleware, MaxBodySize, RateLimiter, CORS, Recovery.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `username` (3-30 chars alphanum), `email`, `first_name`, `last_name`.
// @Description  - Optional fields: `phone` (format E.164, omitempty), `bio` (max 500 chars), `location`, `school`, `work`, `profile_picture_id`.
// @Description  - Validation rules: Contraintes GIN standard (tailles et formats).
// @Description
// @Description  **Execution Workflow:**
// @Description  1. Validation du payload JSON entrant (`c.ShouldBindJSON`).
// @Description  2. Récupération en cascade (L2 MongoDB -> L3 PostgreSQL) du profil actuel pour fusionner avec les champs protégés (Grade, Statuts de ban, etc.).
// @Description  3. Contrôle d'unicité (Filtre Cuckoo -> L1 -> L2 -> L3) si `username`, `email`, ou `phone` ont été modifiés.
// @Description     - En cas de changement, les flags de vérification (`EmailVerified`, `PhoneVerified`) sont réinitialisés à `false`.
// @Description  4. Gestion asynchrone des médias (Out-of-band) : Si un nouveau `profile_picture_id` est soumis, l'ancien est désactivé et le nouveau est validé/activé via le `media_service`.
// @Description  5. Rafraîchissement immédiat de la vue utilisateur dans le cache RAM applicatif (L1 Speed Cache).
// @Description  6. Mise à jour en tâche de fond du filtre probabiliste (Cuckoo Filter) si les identifiants d'unicité ont muté.
// @Description  7. Notification temps réel diffusée en WebSocket (`NotificationProfileUpdated`) vers les instances actives de l'utilisateur.
// @Description  8. Mise en file d'attente (Write-Behind vers Redis) de la nouvelle entité User pour persistance finale en L2 et L3.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: `auth_models.UpdateProfileOutput` contenant le timestamp de dernière modification.
// @Description  - Persistence guarantees: Modifications asynchrones en base (MongoDB/PostgreSQL) par Write-Behind Redis, mais immédiatement disponibles en RAM.
// @Description  - Side effects: Les caches L1 sont purgés/remplacés, WebSockets notifiés, Cuckoo filters mis à jour en tâche de fond. L'ancien média est marqué inactif.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description
// @Description  - **[INVALID_PAYLOAD] Invalid validation rules:**
// @Description    - Trigger: Le corps JSON manque de champs obligatoires, contient un format invalide, ou l'identifiant du nouveau média n'est pas validable.
// @Description    - Execution stage: Validation GIN ou Validation d'activation du Média.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInvalidPayload` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  🟠 **404 Not Found:**
// @Description
// @Description  - **[NOT_FOUND] User profile vanished:**
// @Description    - Trigger: Le compte a été effacé physiquement pendant l'opération ou est corrompu en base de données.
// @Description    - Execution stage: Chargement initial L2/L3.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeNotFound` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  🟠 **409 Conflict:**
// @Description
// @Description  - **[CONFLICT] Field already in use:**
// @Description    - Trigger: Le `username`, `email` ou `phone` soumis est déjà attribué à un autre compte.
// @Description    - Execution stage: Vérification d'unicité.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeConflict` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description
// @Description  - **[INTERNAL_ERROR] Lookup failure:**
// @Description    - Trigger: Échec de connexion à PostgreSQL (L3) lors du chargement de l'ancien profil.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInternalError` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Tags         settings
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   user_settings_models.UpdateProfileInput true "Données de mise à jour du profil"
// @Success      200  {object}  user_settings_models.UpdateProfileOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload or Media Error"
// @Failure      404  {object}  numan_error.PublicErrorResponse "User Not Found"
// @Failure      409  {object}  numan_error.PublicErrorResponse "Conflict (Email/Phone/Username taken)"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /settings/profile/update [put]
func UpdateProfileHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input user_settings_models.UpdateProfileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide.", err))
		return
	}
	output, err := user_settings_service.UpdateProfile(c.Request.Context(), callerID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, output)
}
