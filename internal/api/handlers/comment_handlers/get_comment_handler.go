package comment_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/gin-gonic/gin"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/comment_service"
)

// GetCommentsHandler godoc
// @Summary      Récupérer les commentaires d'une publication
// @Description  Récupère la liste paginée des commentaires d'un post via une stratégie hybride haute performance (ZSET Redis L1, MongoDB L2, PostgreSQL L3).
// @Description  La route inclut une hydratation en cascade pour la résilience et intègre un filtrage complet selon la matrice de visibilité du parent (Abonnés, Amis, Bloqués).
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée (Token JWT obligatoire).
// @Description  - Required permissions or roles: Utilisateur connecté.
// @Description  - Relevant middleware: HMAC Validator, RateLimiter, JWT Auth.
// @Description  - Security checks: Vérifie de manière synchrone l'existence du post parent. Si l'utilisateur n'est pas l'auteur, le système vérifie la relation (Bloqué, Follow, Ami) pour interdire l'accès selon la visibilité du post.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `post_id` (int64 en query param).
// @Description  - Optional fields: `offset` (int, pagination), `limit` (int, défaut 50, bridé à 100).
// @Description  - Validation rules: Binding Query standard.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Validation Synchrone** :
// @Description     - Binding des paramètres Query (`ShouldBindQuery`) et application d'un bouclier de pagination (maximum 100 commentaires).
// @Description  2. **Vérification du Post Parent (Cascade)** :
// @Description     - Recherche du post parent dans le Cache L1, puis L2 (Mongo), et enfin L3 (Postgres). Si chargé depuis L3, déclenche une auto-guérison asynchrone pour re-peupler L1 et L2.
// @Description  3. **Matrice de Visibilité** :
// @Description     - Validation de la relation auteur/utilisateur courant (Bloqué, Réservé Abonnés/Amis, Privé). Interruption immédiate (403) si droits insuffisants.
// @Description  4. **Récupération des Commentaires (Stratégie Hybride)** :
// @Description     - **Tentative L1 (ZSET Redis)** : Pour un faible offset, utilise le ZSET trié par score pour obtenir les identifiants, puis hydrate les enveloppes. Renvoie immédiatement si succès. Les commentaires en "Soft Delete" (Visibilité = -1) remontent avec un message d'erreur localisé.
// @Description     - **Tentative L2 (MongoDB)** : Chargement de l'arbre B-Tree. Si succès, renfloue le ZSET L1 et renvoie les résultats.
// @Description     - **Tentative L3 (PostgreSQL)** : Base de vérité ultime (offset 0 uniquement). Si trouvé, auto-guérison massive (ZSET L1, Object Cache, File Redis vers L2).
// @Description  5. **Réponse** : Renvoi de la liste formatée.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: Liste paginée (`[]comment_models.GetCommentOutput`). Peut être un tableau vide `[]`.
// @Description  - Persistence guarantees: Lecture uniquement. Aucune écriture bloquante.
// @Description  - Side effects: Processus d'auto-guérison asynchrone (Write-Behind vers MongoDB, Hydratation L1) si un cache-miss (L2/L3) s'est produit.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description  - **[INVALID_QUERY] Paramètres invalides:**
// @Description    - Trigger: Le paramètre `post_id` est manquant dans l'URL ou n'est pas un nombre.
// @Description    - Execution stage: Validation GIN (`ShouldBindQuery`).
// @Description    - Response: `numan_error.PublicErrorResponse` avec détails.
// @Description    - Error code: "INVALID_QUERY" (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  🟠 **401 Unauthorized:**
// @Description  - **[UNAUTHORIZED] Authentification échouée:**
// @Description    - Trigger: Le token JWT est absent, expiré, ou la signature HMAC est invalide.
// @Description    - Execution stage: Middleware JWT / Extraction Context.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeUnauthorized` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  🔴 **403 Forbidden:**
// @Description  - **[FORBIDDEN_ACCESS] Droit de lecture refusé:**
// @Description    - Trigger: L'utilisateur a été bloqué par l'auteur, ou le post est réservé aux amis/abonnés et la relation actuelle est insuffisante.
// @Description    - Execution stage: Vérification de la matrice de visibilité.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeForbidden` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  ⚫ **404 Not Found:**
// @Description  - **[RESOURCE_NOT_FOUND] Post parent introuvable:**
// @Description    - Trigger: Le `post_id` demandé n'existe dans aucun cache (L1/L2) ni dans la base maître L3 (ou a été supprimé).
// @Description    - Execution stage: Fallback L3 (PostgreSQL).
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeNotFound` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description  - **[INTERNAL_SERVER_ERROR] Échec L3:**
// @Description    - Trigger: Erreur d'accès inattendue à la base PostgreSQL lors de la cascade.
// @Description    - Execution stage: Lecture Fallback L3.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInternalError` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Tags         comments
// @Accept       json
// @Produce      json
// @Param        Authorization header string true  "Bearer <votre_jwt>"
// @Param        X-Signature   header string true  "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true  "Timestamp Unix de la requête"
// @Param        input         body   comment_models.GetCommentsInput true "Paramètres de requête pour la récupération des commentaires (post_id obligatoire, limit et offset optionnels)"
// @Success      200  {array}   comment_models.GetCommentOutput "Liste paginée et triée des commentaires"
// @Failure      400  {object}  numan_error.PublicErrorResponse "Paramètre manquant ou format invalide"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Session expirée ou utilisateur non identifié"
// @Failure      403  {object}  numan_error.PublicErrorResponse "Accès refusé par la matrice de visibilité"
// @Failure      404  {object}  numan_error.PublicErrorResponse "Post parent introuvable"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Erreur interne lors de la récupération des données"
// @Router       /comment/get [post]
func GetCommentsHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input comment_models.GetCommentsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou champs manquants.", err))
		return
	}

	comments, err := comment_service.GetComments(c.Request.Context(), callerID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, comments)
}
