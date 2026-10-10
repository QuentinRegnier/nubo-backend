package auth_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/service/auth_service"
	"github.com/gin-gonic/gin"
)

// SignUpHandler godoc
// @Summary      Créer un compte utilisateur
// @Description  Exécute l'inscription complète d'un utilisateur sur la plateforme numan et établit sa première session.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Aucune (Route publique).
// @Description  - Required permissions or roles: Aucune.
// @Description  - Relevant middleware: MaxBodySize, RateLimiter, CORS, Recovery.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `username` (3-30 chars, alphanum), `email` (format email), `phone` (format E.164), `password_hash` (min 8 chars), `birthdate` (8 chars, numérique), `firebase_installation_id`.
// @Description  - Optional fields: `first_name`, `last_name`, `gender` (0, 1 ou 2), `bio`, `location`, `school`, `work`, `device_info`, `profile_picture_id` (média préalablement uploadé), ainsi que les paramètres optionnels `display_and_content`, `privacy` et `notifications`.
// @Description  - Validation rules: L'âge calculé depuis la date de naissance doit être compris entre 13 et 120 ans. Le binding GIN valide les contraintes de taille.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. Validation du JSON entrant (`c.ShouldBindJSON`).
// @Description  2. Vérification d'unicité sur le `username`, `email`, et `phone` (Interrogation de la base).
// @Description  3. Validation des contraintes métier sur la date de naissance et le genre.
// @Description  4. Si fourni, validation et activation du média correspondant au `profile_picture_id`.
// @Description  5. Génération asynchrone des identifiants métier (User, Profil, Settings, Session) via un générateur Snowflake.
// @Description  6. Mise en cache immédiate (L1 RAM) et marquage de la timeline comme vide pour initialiser le compte.
// @Description  7. File d'attente (Write-Behind vers Redis) pour la persistance asynchrone dans MongoDB (L2) et PostgreSQL (L3) des 4 entités créées.
// @Description  8. Génération du `MasterToken` et `JWT` cryptographiques liant l'appareil.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: `auth_models.SignUpResponse` renvoyant le UserID, les tokens d'authentification, le message de succès, l'avatar généré, et la structure de télémétrie par défaut.
// @Description  - Persistence guarantees: Délégation intégrale via Write-Behind Redis. L'utilisateur est utilisable en mémoire mais sa persistance définitive est asynchrone.
// @Description  - Side effects: Activation du média de profil, initialisation du cache Timeline, file d'attente remplie.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description
// @Description  - **[INVALID_PAYLOAD] Invalid validation rules:**
// @Description    - Trigger: Le corps JSON manque de champs obligatoires, une contrainte (taille, âge < 13 ans ou > 120 ans, format de date erroné) n'est pas respectée, ou le média de profil n'est pas activable.
// @Description    - Required request/state: Corps de requête invalide ou données incohérentes.
// @Description    - Execution stage: Validation GIN ou Validation métier des champs (âge, média).
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInvalidPayload` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  🟠 **409 Conflict:**
// @Description
// @Description  - **[CONFLICT] Resource already exists:**
// @Description    - Trigger: Le `username`, `email`, ou `phone` existe déjà dans le système.
// @Description    - Required request/state: Donnée en doublon.
// @Description    - Execution stage: Vérification d'unicité asynchrone L2/L3.
// @Description    - Response: `numan_error.PublicErrorResponse` avec un message désignant le champ en conflit.
// @Description    - Error code: `numan_error.CodeConflict` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description
// @Description  - **[INTERNAL_ERROR] Token Generation failure:**
// @Description    - Trigger: Échec du système cryptographique (MasterToken ou JWT).
// @Description    - Required request/state: Panne serveur.
// @Description    - Execution stage: Génération de tokens post-création.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInternalError` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        input         body   auth_models.SignUpInput true "Données d'inscription de l'utilisateur"
// @Success      200  {object}  auth_models.SignUpResponse
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload or Age Restriction"
// @Failure      409  {object}  numan_error.PublicErrorResponse "Conflict (Username, Email, Phone taken)"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /signup [post]
func SignUpHandler(c *gin.Context) {

	var input auth_models.SignUpInput

	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	response, err := auth_service.CreateUser(c.Request.Context(), input, c.ClientIP())
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}
