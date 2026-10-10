package comment_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/gin-gonic/gin"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/comment_service"
)

// DeleteCommentHandler godoc
// @Summary      Supprimer un commentaire
// @Description  Effectue un "Soft Delete" d'un commentaire (visibilité = -1) et décrémente instantanément le compteur du post parent en arrière-plan.
// @Description  Purge instantanément le commentaire de l'Object Cache (L1).
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée (Token JWT obligatoire).
// @Description  - Required permissions or roles: L'utilisateur doit être l'auteur du commentaire.
// @Description  - Relevant middleware: HMAC Validator, RateLimiter, JWT Auth.
// @Description  - Security checks: Le `security_service.LeftComment` vérifie que le commentaire existe et que l'utilisateur en est le propriétaire. Si le commentaire n'existe pas ou appartient à un autre utilisateur, l'action est rejetée.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `comment_id` (int64, identifiant unique du commentaire).
// @Description  - Validation rules: Binding JSON standard.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Validation Synchrone** :
// @Description     - Extraction de l'ID utilisateur via le middleware JWT (`pkg.GetUserIDFromContext`).
// @Description     - Binding du payload JSON pour extraire `comment_id`.
// @Description  2. **Vérification de Sécurité (Service)** :
// @Description     - Le service `security_service.LeftComment` localise le commentaire (via Cache L1 ou base de données) et vérifie l'identité de l'auteur.
// @Description  3. **Purge et Modification (Service)** :
// @Description     - Suppression immédiate de l'enveloppe du commentaire dans l'Object Cache (L1).
// @Description     - Suppression de l'identifiant du commentaire dans le ZSET Redis du post parent.
// @Description     - Décrémentation instantanée de `CommentCount` sur le Post parent en cache L1 et recalcul de son score de recommandation.
// @Description     - Le payload du commentaire est passé en visibilité `-1` (Soft Delete).
// @Description  4. **Délégation Asynchrone** :
// @Description     - Le commentaire modifié (ActionDelete) est poussé dans la file d'attente Redis Write-Behind. Les workers propageront le Soft Delete et la décrémentation du parent vers MongoDB et PostgreSQL de manière transactionnelle.
// @Description  5. **Réponse Immédiate** : Renvoi du statut 200 OK.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: `{"message": "Commentaire supprimé avec succès"}`
// @Description  - Persistence guarantees: Suppression synchrone du Cache L1, puis délégation asynchrone (Redis Write-Behind) garantissant la cohérence éventuelle (Soft Delete en L2/L3).
// @Description  - Side effects: Purge du ZSET, décrémentation de `CommentCount` sur le post parent, baisse du score de recommandation du post.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description  - **[INVALID_PAYLOAD] Invalid format or validation failed:**
// @Description    - Trigger: Le format JSON est invalide, ou le champ `comment_id` est manquant.
// @Description    - Execution stage: Validation GIN.
// @Description    - Response: `numan_error.PublicErrorResponse` avec détails.
// @Description    - Error code: `numan_error.CodeInvalidPayload` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  🟠 **401 Unauthorized:**
// @Description  - **[UNAUTHORIZED] Authentification échouée:**
// @Description    - Trigger: Le token JWT est absent, expiré, ou la signature HMAC est invalide.
// @Description    - Execution stage: Middleware JWT / Extraction Context.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeUnauthorized` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  🔴 **403 Forbidden:**
// @Description  - **[FORBIDDEN_ACCESS] Violation des droits d'auteur:**
// @Description    - Trigger: Le commentaire a été trouvé mais n'appartient pas à l'utilisateur courant.
// @Description    - Execution stage: Vérification de sécurité (`LeftComment`).
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeForbidden` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  ⚫ **404 Not Found:**
// @Description  - **[RESOURCE_NOT_FOUND] Commentaire introuvable:**
// @Description    - Trigger: Le `comment_id` ne correspond à aucun commentaire actif.
// @Description    - Execution stage: Vérification de sécurité (`LeftComment`).
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeNotFound` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description  - **[INTERNAL_SERVER_ERROR] Erreur serveur inattendue:**
// @Description    - Trigger: Panne de base de données durant la vérification, ou échec d'insertion dans la file Redis (EnqueueDB).
// @Description    - Execution stage: Accès L2/L3 ou File d'attente Redis.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInternalError` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Tags         comments
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   comment_models.DeleteCommentInput true "ID du commentaire à supprimer"
// @Success      200  {object}  map[string]string "message: Commentaire supprimé avec succès"
// @Failure      400  {object}  numan_error.PublicErrorResponse "Format JSON invalide ou champ manquant"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Session expirée ou utilisateur non identifié"
// @Failure      403  {object}  numan_error.PublicErrorResponse "Violation des droits d'auteur"
// @Failure      404  {object}  numan_error.PublicErrorResponse "Commentaire introuvable"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Erreur interne du serveur"
// @Router       /comment/delete [delete]
func DeleteCommentHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input comment_models.DeleteCommentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou comment_id manquant.", err))
		return
	}

	// Appel au service métier (Cascade L1->L2->L3 & Envoi Asynchrone)
	err = comment_service.DeleteComment(c.Request.Context(), callerID, input)
	if err != nil {
		// Magique : Plus besoin des if err.Error() == "unauthorized" !
		numan_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Commentaire supprimé avec succès"})
}
