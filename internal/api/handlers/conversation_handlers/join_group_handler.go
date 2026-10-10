package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// JoinGroupHandler godoc
// @Summary      Rejoindre un groupe ou une communauté
// @Description  Intègre l'utilisateur connecté dans un groupe ou communauté, sous réserve de la validation des règles métier (lien direct, mot de passe, invitation, bannissement).
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée (Token JWT requis).
// @Description  - Security checks: Validation minutieuse du statut (banni?), du type de groupe (Message Privé interdit), des permissions, et des éventuelles invitations requises.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `conversation_id`.
// @Description  - Optional fields: `message_id` (s'il rejoint via un message d'invitation).
// @Description
// @Description  **Execution Workflow:**
// @Description  1. Le service valide l'existence du groupe. Rejet (403) si la conversation est un Message Privé.
// @Description  2. Analyse des modes d'accès du groupe: public (entrée libre), interne, ou sur approbation stricte.
// @Description  3. Validation d'un lien d'invitation externe ou d'un message d'invitation (Type 6) s'il a été fourni. Vérifie que le lien correspond et n'a pas expiré.
// @Description  4. Le statut du membre (banni du groupe) et l'existence d'une demande pendante (Pending) sont inspectés.
// @Description  5. Si toutes les autorisations concordent, le `MemberPayload` est créé.
// @Description  6. Persistance Write-Behind et diffusion asynchrone WebSocket aux membres de l'arrivée du nouvel utilisateur.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: `conversation_models.JoinGroupOutput`.
// @Description  - Persistence guarantees: Cache L1 synchrone, Base de données via Redis Write-Behind.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description  - **[INVALID_PAYLOAD] Invitation ou statut invalide:**
// @Description    - **Trigger:** Le format est invalide, l'utilisateur est déjà membre, ou l'invitation transmise n'est pas un message valide.
// @Description    - **Execution stage:** Phase de binding GIN et validation initiale des entrées, avant l'exécution de la logique métier.
// @Description    - **Response:** `numan_error.PublicErrorResponse` contenant le message d'erreur.
// @Description    - **Error code:** `numan_error.CodeInvalidPayload` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  🟠 **401 Unauthorized:**
// @Description  - **[UNAUTHORIZED]**
// @Description    - **Trigger:** Le token JWT est absent de l'en-tête Authorization, mal formaté, expiré ou invalide.
// @Description    - **Execution stage:** Middleware d'authentification JWT, avant que la requête n'atteigne le handler.
// @Description    - **Response:** `numan_error.PublicErrorResponse` contenant le message d'erreur.
// @Description    - **Error code:** `numan_error.CodeUnauthorized` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  🔴 **403 Forbidden:**
// @Description  - **[FORBIDDEN_ACCESS] Intégration interdite:**
// @Description    - **Trigger:** Le groupe est privé et requiert une invitation, le lien a expiré, l'utilisateur est banni, ou c'est un message privé (injoignable).
// @Description    - **Execution stage:** Évaluation des permissions métier dans le handler ou le service appelé, après authentification.
// @Description    - **Response:** `numan_error.PublicErrorResponse` contenant le message d'erreur.
// @Description    - **Error code:** `numan_error.CodeForbidden` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  ⚫ **404 Not Found:**
// @Description  - **[RESOURCE_NOT_FOUND] Groupe introuvable:**
// @Description    - **Trigger:** La conversation ciblée n'existe pas.
// @Description    - **Execution stage:** Tentative de chargement de la ressource dans le cache L1 ou la base de données L2/L3.
// @Description    - **Response:** `numan_error.PublicErrorResponse` contenant le message d'erreur.
// @Description    - **Error code:** `numan_error.CodeNotFound` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  🟠 **409 Conflict:**
// @Description  - **[RESOURCE_CONFLICT] Demande en attente:**
// @Description    - **Trigger:** L'utilisateur a déjà une requête d'adhésion en cours de validation.
// @Description    - **Execution stage:** Exécution de la logique de service.
// @Description    - **Response:** `numan_error.PublicErrorResponse` contenant le message d'erreur.
// @Description    - **Error code:** `numan_error.CodeConflict` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description  - **[INTERNAL_SERVER_ERROR]**
// @Description    - **Trigger:** Une erreur inattendue est survenue lors du traitement, par exemple une perte de connexion avec la base de données ou le cache Redis.
// @Description    - **Execution stage:** À n'importe quel point du traitement interne, généralement lors d'un appel à un service externe ou une base de données.
// @Description    - **Response:** `numan_error.PublicErrorResponse` contenant le message d'erreur.
// @Description    - **Error code:** `numan_error.CodeInternalError` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Tags         conversations
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   conversation_models.JoinGroupInput true "Payload pour rejoindre un groupe ou une communauté"
// @Success      200  {object}  conversation_models.JoinGroupOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Unauthorized"
// @Failure      403  {object}  numan_error.PublicErrorResponse "Forbidden"
// @Failure      404  {object}  numan_error.PublicErrorResponse "Not Found"
// @Failure      409  {object}  numan_error.PublicErrorResponse "Conflict"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Error"
// @Router       /group/join [post]
func JoinGroupHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input conversation_models.JoinGroupInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Le JSON de la requête est mal formaté.", err))
		return
	}

	output, errService := conversation_service.JoinGroup(c.Request.Context(), callerID, input)
	if errService != nil {
		numan_error.RespondWithError(c, errService)
		return
	}

	c.JSON(http.StatusOK, output)
}
