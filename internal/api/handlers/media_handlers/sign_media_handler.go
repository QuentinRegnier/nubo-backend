package media_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/service/media_service"
	"github.com/gin-gonic/gin"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
)

// SignMediaHandler godoc
// @Summary      Générer une URL sécurisée (Signée)
// @Description  Génère une URL signée (Sceau Cryptographique HMAC) pour autoriser un client mobile/web à accéder à un média privé sur le CDN. Évalue un accès Zero-Trust en fonction du contexte fourni.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée (Token JWT requis). L'accès public (avatars) est techniquement validé via `context_id=0`.
// @Description  - Required permissions or roles: Pour une image dans un Post, vérifie l'accès au post (Blocages, Visibilité). Pour un message, vérifie la participation à la Conversation (Rôle > 0).
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `media_id` (int64).
// @Description  - Optional fields: Soit `post_id` (int64) soit `conversation_id` (int64) selon le contexte de l'image.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Validation** : Vérifie qu'il n'y a pas d'ambiguïté (On ne peut pas fournir à la fois un `post_id` ET un `conversation_id`).
// @Description  2. **Hydratation du média** : Le média ciblé est cherché en base L1/L2/L3. S'il n'existe pas ou n'est plus visible, rejet 404.
// @Description  3. **Matrice de Sécurité** :
// @Description     - Si `post_id` fourni : Appelle `security_service.LeftPost` pour vérifier l'accès au post, puis confirme fermement que le `media_id` appartient bien à ce post (Anti-Usurpation).
// @Description     - Si `conversation_id` fourni : Appelle `security_service.LeftMember` pour s'assurer que l'utilisateur est bien membre de cette conversation.
// @Description     - Si rien de fourni : Accepte l'accès, considéré comme un Avatar public (Contexte 0).
// @Description  4. **Signature HMAC** : Crée un sceau d'intégrité associant le chemin du média, l'id du propriétaire, le contexte et l'id du requérant (sans durée limite stricte dans ce fragment de code de la signature).
// @Description  5. **Réponse** : L'URL signée prête à être utilisée est renvoyée.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: `media_models.SignMediaOutput` (avec `MediaID` et la `URL` signée).
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request — Payload invalide ou ambigu**
// @Description  - **Trigger:** Le payload est invalide ou bien le client spécifie à la fois `post_id` et `conversation_id` (Contextes mutuellement exclusifs).
// @Description  - **Execution stage:** Binding GIN ou Bouclier anti-ambiguïté (Étape 3 du handler).
// @Description  - **Response:** Structure JSON `numan_error.PublicErrorResponse`.
// @Description  - **Error code:** `numan_error.CodeInvalidPayload` (si binding) ou `AMBIGUOUS_CONTEXT` (Littéral exact) renvoyé dans le champ JSON `code`.
// @Description
// @Description  🟠 **401 Unauthorized — Authentification échouée**
// @Description  - **Trigger:** Le token JWT est absent, invalide ou expiré.
// @Description  - **Execution stage:** Middleware d'authentification ou extraction de profil.
// @Description  - **Response:** Structure JSON `numan_error.PublicErrorResponse`.
// @Description  - **Error code:** `numan_error.CodeUnauthorized` (renvoyé dans le champ JSON `code`).
// @Description
// @Description  🔴 **403 Forbidden — Accès non autorisé au média ou usurpation**
// @Description  - **Trigger:** L'utilisateur n'a pas accès au post (`security_service.LeftPost`), n'est pas membre du groupe, ou tente d'usurper l'accès en demandant un média qui n'est pas rattaché au `post_id` fourni.
// @Description  - **Execution stage:** Validation au sein de la Matrice de Sécurité Contextuelle dans le service `GetSignedURLForClient`.
// @Description  - **Response:** Structure JSON `numan_error.PublicErrorResponse`.
// @Description  - **Error code:** `numan_error.CodeForbidden` (renvoyé dans le champ JSON `code`).
// @Description
// @Description  ⚫ **404 Not Found — Média ou Contexte introuvable**
// @Description  - **Trigger:** Le Média n'existe pas ou sa visibilité est fausse, ou le post/conversation contextuel est inexistant ou introuvable.
// @Description  - **Execution stage:** Chargement cascade du média (L1/L2/L3) ou validation d'accès sécurisé (`security_service`).
// @Description  - **Response:** Structure JSON `numan_error.PublicErrorResponse`.
// @Description  - **Error code:** `numan_error.CodeNotFound` (renvoyé dans le champ JSON `code`).
// @Description
// @Description  ⚫ **500 Internal Server Error — Erreur inattendue**
// @Description  - **Trigger:** Panne base de données survenue au cours de la vérification de l'existence du Média.
// @Description  - **Execution stage:** Chargement Postgres ou MongoDB interne.
// @Description  - **Response:** Structure JSON `numan_error.PublicErrorResponse`.
// @Description  - **Error code:** `numan_error.CodeInternalError` (renvoyé dans le champ JSON `code`).
// @Tags         media
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   media_models.SignMediaInput true "Payload pour obtention d'une URL signée pour un média privé"
// @Success      200  {object}  media_models.SignMediaOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload ou Ambiguous Context"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Unauthorized"
// @Failure      403  {object}  numan_error.PublicErrorResponse "Forbidden"
// @Failure      404  {object}  numan_error.PublicErrorResponse "Not Found"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /media/sign [post]
func SignMediaHandler(c *gin.Context) {
	// 1. Extraction du ReaderID (l'utilisateur qui demande à voir l'image)
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	// 2. Parsing du payload JSON
	var input media_models.SignMediaInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou paramètre media_id manquant.", err))
		return
	}

	// 3. Appel du service de signature (qui effectue le contrôle Zero-Trust L1->L2->L3)
	output, err := media_service.GetSignedURLForClient(c.Request.Context(), callerID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	// 4. Renvoi du "ticket" valide à l'application mobile
	c.JSON(http.StatusOK, output)
}
