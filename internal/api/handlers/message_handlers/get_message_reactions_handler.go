package message_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/message_service"
	"github.com/gin-gonic/gin"
)

// GetMessageReactionsHandler godoc
// @Summary      Lister les réactions à un message
// @Description  Récupère la liste détaillée des utilisateurs ayant réagi à un message spécifique au sein d'une conversation.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée nécessitant un token JWT d'authentification valide.
// @Description  - Required permissions or roles: L'utilisateur appelant doit être membre actif de la conversation contenant le message.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Validation Synchrone :**
// @Description     - Extraction de l'ID de l'utilisateur requérant depuis le contexte (via le token JWT validé par middleware).
// @Description     - Binding GIN et validation du payload (`input.MessageID`).
// @Description  2. **Contrôle d'Accès :**
// @Description     - Appel à `security_service.RightMessage` pour vérifier que le message existe et que l'appelant est bien membre de la conversation associée.
// @Description  3. **Lecture des Données depuis le Cache :**
// @Description     - Récupération des réactions brutes (`cache_service.GetMessageReactionsRaw`) depuis Redis en O(1) pour la clé du message.
// @Description  4. **Hydratation et Construction de la Réponse :**
// @Description     - Boucle sur chaque réaction pour récupérer le profil allégé de l'utilisateur (`cache_service.GetUserLite`).
// @Description     - Génération d'un lien d'avatar HMAC cryptographiquement signé si l'utilisateur possède une photo de profil (`media_service.GenerateMediaViewCascade`).
// @Description     - Vérification du statut en ligne de l'utilisateur (`cache_service.IsUserOnline`).
// @Description     - Assemblage des vues et renvoi de la liste en format JSON.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description
// @Description  - **[numan_error.CodeInvalidPayload] Requête mal formatée ou paramètres invalides:**
// @Description    - Trigger: Le payload est invalide (ex: MessageID manquant ou mal formaté).
// @Description    - Execution stage: Validation GIN.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInvalidPayload`
// @Description
// @Description  🟡 **401 Unauthorized:**
// @Description
// @Description  - **[numan_error.CodeUnauthorized] Jeton invalide ou absent:**
// @Description    - Trigger: Le client HTTP n'envoie pas de token, ou le JWT a expiré.
// @Description    - Execution stage: Middleware global d'authentification.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeUnauthorized`
// @Description
// @Description  🟠 **403 Forbidden:**
// @Description
// @Description  - **[numan_error.CodeForbidden] Accès refusé:**
// @Description    - Trigger: L'appelant tente d'accéder aux réactions d'un message situé dans une conversation dont il ne fait pas partie.
// @Description    - Execution stage: Évaluation des règles de sécurité (`security_service.RightMessage`).
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeForbidden`
// @Description
// @Description  🟤 **404 Not Found:**
// @Description
// @Description  - **[numan_error.CodeNotFound] Ressource introuvable:**
// @Description    - Trigger: Le message ciblé n'existe pas ou a été supprimé.
// @Description    - Execution stage: Requête de vérification du message en base de données ou cache.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeNotFound`
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description
// @Description  - **[numan_error.CodeInternalError] Erreur de service inattendue:**
// @Description    - Trigger: Panne de la base de données, inaccessibilité de Redis, timeout ou erreur inattendue du service de médias.
// @Description    - Execution stage: Exécution profonde du service métier.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInternalError`
// @Tags         messages
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   message_models.GetMessageReactionsInput true "Payload pour récupérer la liste des réactions à un message spécifique"
// @Success      200  {object}  message_models.GetMessageReactionsOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Unauthorized"
// @Failure      403  {object}  numan_error.PublicErrorResponse "Forbidden"
// @Failure      404  {object}  numan_error.PublicErrorResponse "Not Found"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /messages/reactions/list [post]
func GetMessageReactionsHandler(c *gin.Context) {
	// 1. Récupération sécurisée de l'identité de l'appelant via le middleware
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	// 2. Binding strict du payload JSON (Zéro paramètre d'URL)
	var input message_models.GetMessageReactionsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Le format de la requête est invalide.", err))
		return
	}

	// 3. Appel du service métier pur (Logique et accès aux données)
	output, err := message_service.GetMessageReactions(c.Request.Context(), callerID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	// 4. Retour HTTP structuré
	c.JSON(http.StatusOK, output)
}
