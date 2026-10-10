package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// AddMemberHandler godoc
// @Summary      Ajouter des membres à un groupe
// @Description  Ajoute de nouveaux membres à une conversation de groupe existante ou leur envoie une invitation selon leurs paramètres de confidentialité.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée (Token JWT requis).
// @Description  - Required permissions or roles: Le requérant doit faire partie du groupe. Si les paramètres du groupe l'exigent, le requérant doit être Administrateur.
// @Description  - Relevant middleware: HMAC Validator, RateLimiter, JWT Auth.
// @Description  - Security checks: L'ajout est bloqué si la conversation est un Message Privé individuel (Type 1), si le groupe n'existe pas, ou si le requérant n'a pas les droits nécessaires.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `conversation_id` (int64), `participant_ids` (tableau d'int64).
// @Description  - Validation rules: Binding GIN standard. Le backend évalue ensuite individuellement chaque ID de participant.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Validation & Extraction** : Vérification du payload JSON et récupération de l'identité du requérant.
// @Description  2. **Vérification de la Conversation** : Chargement des paramètres du groupe depuis le cache L1. Refus 403 si c'est un message privé (Type 1) ou si les droits sont insuffisants.
// @Description  3. **Traitement par Candidat** :
// @Description     - Vérification de la relation (Blocage actif, permission `AddGroupPermission`).
// @Description     - **Ajout Direct** : Si le candidat autorise l'ajout direct, un `MemberPayload` est créé (Cache L1, Redis Write-Behind pour L2/L3) et un message système est généré.
// @Description     - **Invitation** : Si le candidat restreint les ajouts, une invitation (Message Type 6) lui est envoyée via un canal privé (Conversation Directe).
// @Description  4. **Notifications & Realtime** : Diffusion asynchrone (`member.added`) aux participants connectés et envoi de notifications push.
// @Description  5. **Mise à jour d'activité** : TouchInboxActivity met à jour le Timestamp L1.
// @Description  6. **Réponse** : Renvoie les identifiants catégorisés (Ajoutés, Invités, Rejetés).
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: `conversation_models.AddMemberOutput` (Listes Added, Invited, Rejected).
// @Description  - Persistence guarantees: Cache L1 synchrone, file Redis Write-Behind pour la base de données.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description  - **[INVALID_PAYLOAD] Format JSON invalide:**
// @Description    - **Trigger:** Le payload ne respecte pas le modèle ou est manquant.
// @Description    - **Execution stage:** Validation GIN.
// @Description    - **Response:** `numan_error.PublicErrorResponse`.
// @Description    - **Error code:** `numan_error.CodeInvalidPayload` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  🟠 **401 Unauthorized:**
// @Description  - **[UNAUTHORIZED] Authentification échouée:**
// @Description    - **Trigger:** Le token JWT est absent ou expiré.
// @Description    - **Execution stage:** Middleware d'authentification JWT, avant que la requête n'atteigne le handler.
// @Description    - **Response:** `numan_error.PublicErrorResponse` contenant le message d'erreur.
// @Description    - **Error code:** `numan_error.CodeUnauthorized` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  🔴 **403 Forbidden:**
// @Description  - **[FORBIDDEN_ACCESS] Ajout interdit:**
// @Description    - **Trigger:** La conversation est un message privé (Type 1), ou l'ajout est restreint aux administrateurs et le requérant est un membre normal.
// @Description    - **Execution stage:** Vérification des règles de conversation.
// @Description    - **Response:** `numan_error.PublicErrorResponse` contenant le message d'erreur.
// @Description    - **Error code:** `numan_error.CodeForbidden` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  ⚫ **404 Not Found:**
// @Description  - **[RESOURCE_NOT_FOUND] Conversation introuvable:**
// @Description    - **Trigger:** L'ID du groupe n'existe pas ou le requérant n'en fait pas partie.
// @Description    - **Execution stage:** Tentative de chargement de la ressource dans le cache L1 ou la base de données L2/L3.
// @Description    - **Response:** `numan_error.PublicErrorResponse` contenant le message d'erreur.
// @Description    - **Error code:** `numan_error.CodeNotFound` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Tags         conversations
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input 		   body   conversation_models.AddMemberInput true "Payload d'ajout"
// @Success      200  {object}  conversation_models.AddMemberOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Unauthorized"
// @Failure      403  {object}  numan_error.PublicErrorResponse "Forbidden"
// @Failure      404  {object}  numan_error.PublicErrorResponse "Not Found"
// @Router       /group/user/set [post]
func AddMemberHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input conversation_models.AddMemberInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	output, err := conversation_service.AddMembersToConversation(c.Request.Context(), callerID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, output)
}
