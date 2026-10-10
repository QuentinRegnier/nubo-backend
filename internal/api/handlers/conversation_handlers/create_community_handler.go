package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// CreateCommunityHandler godoc
// @Summary      Créer une communauté publique
// @Description  Crée une nouvelle communauté (Conversation Type 3). Seuls les utilisateurs avec un Grade suffisant peuvent effectuer cette action, ou des administrateurs pour le compte de tiers.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée (Token JWT requis).
// @Description  - Required permissions or roles: L'utilisateur doit avoir un grade adéquat ou ne pas avoir atteint son quota de communautés, sauf si délégué par un Modérateur/Administrateur système.
// @Description  - Relevant middleware: HMAC Validator, RateLimiter, JWT Auth.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `title` (string).
// @Description  - Optional fields: `description`, `link`, `owner_id` (pour délégation d'admin), etc.
// @Description  - Validation rules: Le titre est obligatoire. Si `owner_id` est spécifié et différent du requérant, le requérant doit être modérateur/administrateur.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Validation Synchrone** : Extraction JWT et Binding JSON.
// @Description  2. **Vérification des Quotas (Service)** : Contrôle du Grade utilisateur. Rejet (403) si le grade est insuffisant ou le quota atteint.
// @Description  3. **Vérification de Délégation** : Si l'owner cible est différent, vérification stricte du rôle du requérant.
// @Description  4. **Initialisation** : Création du `ConversationPayload` (Type 3) et du `MemberPayload` pour le propriétaire avec le rôle Admin.
// @Description  5. **Indexation globale** : Insertion dans le ZSET des recommandations globales (L1).
// @Description  6. **Persistance Asynchrone** : Redis Write-Behind (`ActionCreate` sur `EntityConversation` et `EntityMembers`).
// @Description  7. **Réponse** : Renvoi de l'ID de la nouvelle communauté.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: `conversation_models.CreateCommunityOutput` (incluant `conversation_id`).
// @Description  - Persistence guarantees: Cache L1 (immédiat), Redis Write-Behind pour L2/L3.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description  - **[INVALID_PAYLOAD] Requête invalide:**
// @Description    - **Trigger:** Le payload est invalide, ou l'utilisateur cible désigné n'existe pas.
// @Description    - **Execution stage:** Phase de binding GIN et validation initiale des entrées, avant l'exécution de la logique métier.
// @Description    - **Response:** `numan_error.PublicErrorResponse` contenant le message d'erreur.
// @Description    - **Error code:** `numan_error.CodeInvalidPayload` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  🟠 **401 Unauthorized:**
// @Description  - **[UNAUTHORIZED] Authentification échouée:**
// @Description    - **Trigger:** Le token JWT est absent de l'en-tête Authorization, mal formaté, expiré ou invalide.
// @Description    - **Execution stage:** Middleware d'authentification JWT, avant que la requête n'atteigne le handler.
// @Description    - **Response:** `numan_error.PublicErrorResponse` contenant le message d'erreur.
// @Description    - **Error code:** `numan_error.CodeUnauthorized` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  🔴 **403 Forbidden:**
// @Description  - **[FORBIDDEN_ACCESS] Droits insuffisants ou Quota atteint:**
// @Description    - **Trigger:** Grade insuffisant, quota dépassé, ou tentative de délégation sans droits admin.
// @Description    - **Execution stage:** Évaluation des permissions métier dans le handler ou le service appelé, après authentification.
// @Description    - **Response:** `numan_error.PublicErrorResponse` contenant le message d'erreur.
// @Description    - **Error code:** `numan_error.CodeForbidden` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  ⚫ **404 Not Found:**
// @Description  - **[RESOURCE_NOT_FOUND] Profil introuvable:**
// @Description    - **Trigger:** Le profil du demandeur ou du propriétaire ciblé n'existe plus.
// @Description    - **Execution stage:** Tentative de chargement de la ressource dans le cache L1 ou la base de données L2/L3.
// @Description    - **Response:** `numan_error.PublicErrorResponse` contenant le message d'erreur.
// @Description    - **Error code:** `numan_error.CodeNotFound` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Tags         conversations
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   conversation_models.CreateCommunityInput true "Payload de création"
// @Success      200  {object}  conversation_models.CreateCommunityOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Unauthorized"
// @Failure      403  {object}  numan_error.PublicErrorResponse "Forbidden"
// @Failure      404  {object}  numan_error.PublicErrorResponse "Not Found"
// @Router       /conversation/community/set [post]
func CreateCommunityHandler(c *gin.Context) {
	// 1. Extraction sécurisée de l'ID utilisateur
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	// 2. Parsing du Body
	var input conversation_models.CreateCommunityInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Le format des données est invalide.", err))
		return
	}

	// 3. Appel du service métier pur
	output, err := conversation_service.CreateCommunity(c.Request.Context(), callerID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	// 4. Réponse
	c.JSON(http.StatusCreated, output)
}
