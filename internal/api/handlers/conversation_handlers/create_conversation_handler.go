package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// CreateConversationHandler godoc
// @Summary      Créer une conversation
// @Description  Initialise un message privé ou un groupe privé (Type 1 ou 2).
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée (Token JWT requis).
// @Description  - Security checks: Valide la matrice relationnelle pour s'assurer que les cibles autorisent les messages privés du requérant (vérification de blocage et paramètres de confidentialité).
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `participant_ids` (tableau d'int64, minimum 1). Pour les groupes, `title` est obligatoire.
// @Description  - Validation rules: Création avec soi-même interdite.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Binding & Validation** : Rejet (400) si le groupe n'a pas de titre ou si l'utilisateur tente de discuter avec lui-même.
// @Description  2. **Analyse Relationnelle** : Pour un message privé (1 cible), le système vérifie si la cible a bloqué le requérant ou refuse les DMs. Si oui -> 403 Forbidden.
// @Description  3. **Création du Modèle** : Génération de l'ID, hydratation du `ConversationPayload` et création des `MemberPayload` pour chaque membre (ajoutant le rôle Admin au créateur pour les groupes).
// @Description  4. **Persistance Write-Behind** : Injections dans Redis EnqueueDB pour sauvegarde asynchrone dans MongoDB (L2) et PostgreSQL (L3).
// @Description  5. **Propagation Realtime** : Envoi asynchrone des notifications WebSocket `conversation.created` à tous les membres.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: `conversation_models.CreateConversationOutput`.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description  - **[INVALID_PAYLOAD] Validation échouée:**
// @Description    - **Trigger:** JSON invalide, titre manquant pour un groupe, ou tentative de discussion solo.
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
// @Description  - **[FORBIDDEN_ACCESS] Blocage ou paramètres de confidentialité:**
// @Description    - **Trigger:** La cible a bloqué le requérant ou refuse les messages de non-amis.
// @Description    - **Execution stage:** Évaluation des permissions métier dans le handler ou le service appelé, après authentification.
// @Description    - **Response:** `numan_error.PublicErrorResponse` contenant le message d'erreur.
// @Description    - **Error code:** `numan_error.CodeForbidden` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  ⚫ **404 Not Found:**
// @Description  - **[RESOURCE_NOT_FOUND] Utilisateur cible introuvable:**
// @Description    - **Trigger:** L'ID d'un participant n'existe pas.
// @Description    - **Execution stage:** Tentative de chargement de la ressource dans le cache L1 ou la base de données L2/L3.
// @Description    - **Response:** `numan_error.PublicErrorResponse` contenant le message d'erreur.
// @Description    - **Error code:** `numan_error.CodeNotFound` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description  - **[INTERNAL_SERVER_ERROR]**
// @Description    - **Trigger:** Échec inattendu lors du traitement de la file d'attente.
// @Description    - **Execution stage:** À n'importe quel point du traitement interne, généralement lors d'un appel à un service externe ou une base de données.
// @Description    - **Response:** `numan_error.PublicErrorResponse` contenant le message d'erreur.
// @Description    - **Error code:** `numan_error.CodeInternalError` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Tags         conversations
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   conversation_models.CreateConversationInput true "Payload de création"
// @Success      200  {object}  conversation_models.CreateConversationOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Unauthorized"
// @Failure      403  {object}  numan_error.PublicErrorResponse "Forbidden"
// @Failure      404  {object}  numan_error.PublicErrorResponse "Not Found"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Error"
// @Router       /conversation/create [post]
func CreateConversationHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input conversation_models.CreateConversationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	output, err := conversation_service.CreateConversation(c.Request.Context(), callerID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusCreated, output)
}
