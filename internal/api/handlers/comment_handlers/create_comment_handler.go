package comment_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/gin-gonic/gin"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/comment_service"
)

// CreateCommentHandler godoc
// @Summary      Créer un commentaire
// @Description  Ajoute un commentaire texte à une publication existante.
// @Description  Le traitement est asynchrone (Fire-and-Forget) pour garantir une latence minimale au client (~1ms).
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée (Token JWT obligatoire).
// @Description  - Required permissions or roles: Utilisateur connecté.
// @Description  - Relevant middleware: HMAC Validator, RateLimiter, JWT Auth.
// @Description  - Security checks: L'existence de la publication cible, la matrice de visibilité (Bloqué, Amis seulement) et le statut de l'utilisateur (Banni) sont validés silencieusement par les workers asynchrones avant la persistance finale. Un commentaire illégal est détruit par le pare-feu.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `post_id` (int64, identifiant de la publication ciblée), `content` (string, texte du commentaire).
// @Description  - Validation rules: Le contenu est nettoyé (`pkg.CleanStr`). S'il est vide après nettoyage (uniquement des espaces), l'API renvoie une erreur `EMPTY_COMMENT`.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Validation Synchrone** :
// @Description     - Extraction de l'ID utilisateur via le middleware JWT (`pkg.GetUserIDFromContext`).
// @Description     - Binding du payload JSON et validation du format.
// @Description     - Nettoyage du texte et blocage immédiat si le commentaire est vide.
// @Description  2. **Délégation Asynchrone (Service)** :
// @Description     - Évaluation de l'autorité de l'auteur via le Speed Cache L1 (Grade de l'utilisateur) pour attribuer un score de visibilité initial (PriorityMultiplier).
// @Description     - Génération de l'identifiant Snowflake (`pkg.GenerateID`).
// @Description     - Si la publication est en Object Cache L1 :
// @Description       - Indexation immédiate du commentaire dans le ZSET Redis.
// @Description       - Mise en cache L1 de l'enveloppe du commentaire.
// @Description       - Incrémentation en temps réel du compteur `CommentCount` du Post parent et mise à jour de son score de recommandation.
// @Description     - Mise en file d'attente (Redis Write-Behind Queue) pour persistance finale sur MongoDB (L2) et PostgreSQL (L3).
// @Description     - Envoi d'un événement asynchrone au `notification_service` pour alerter l'auteur du post.
// @Description  3. **Réponse Immédiate** : Renvoi du statut 200 OK.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: `{"message": "Commentaire en cours de publication"}`
// @Description  - Persistence guarantees: Mise à jour immédiate du Cache L1 (si le parent y est présent), puis File d'Attente Redis (Write-Behind) pour une durabilité L2/L3 asynchrone assurée par les workers.
// @Description  - Side effects: Incrémentation du `CommentCount` du post, recalcul du score de recommandation du post, indexation ZSET du commentaire, déclenchement d'une notification `EventCommentAdded`.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description  - **[INVALID_PAYLOAD] Invalid format or validation failed:**
// @Description    - Trigger: Le format JSON est invalide, ou un champ obligatoire comme `post_id` est manquant/invalide.
// @Description    - Execution stage: Validation GIN.
// @Description    - Response: `numan_error.PublicErrorResponse` avec détails.
// @Description    - Error code: `numan_error.CodeInvalidPayload` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description  - **[EMPTY_COMMENT] Erreur métier:**
// @Description    - Trigger: Le champ `content` est absent ou se résume à des espaces après nettoyage.
// @Description    - Execution stage: Traitement métier (Validation synchrone).
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: "EMPTY_COMMENT" (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  🟠 **401 Unauthorized:**
// @Description  - **[UNAUTHORIZED] Authentification échouée:**
// @Description    - Trigger: Le token JWT est absent, expiré, ou la signature HMAC est invalide.
// @Description    - Execution stage: Middleware JWT / Extraction Context.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeUnauthorized` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Tags         comments
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input 		   body   comment_models.CreateCommentInput true "Données du commentaire"
// @Success      200  {object}  map[string]string "message: Commentaire en cours de publication"
// @Failure      400  {object}  numan_error.PublicErrorResponse "Format JSON invalide, post_id manquant ou contenu vide"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Session expirée ou utilisateur non identifié"
// @Router       /comment/set [post]
func CreateCommentHandler(c *gin.Context) {
	// 1. Sécurité : Extraction de l'ID via JWT
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	// 2. Parsing du JSON
	var input comment_models.CreateCommentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou champs manquants.", err))
		return
	}

	// 3. Envoi au Service Asynchrone
	err = comment_service.CreateComment(c.Request.Context(), callerID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	// 6. Confirmation immédiate (Latence ~1ms)
	c.JSON(http.StatusOK, gin.H{"message": "Commentaire en cours de publication"})
}
