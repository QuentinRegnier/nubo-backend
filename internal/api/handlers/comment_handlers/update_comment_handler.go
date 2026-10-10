package comment_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/gin-gonic/gin"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/comment_service"
)

// UpdateCommentHandler godoc
// @Summary      Modifier un commentaire
// @Description  Met à jour le contenu texte d'un commentaire existant.
// @Description  La persistance est gérée de manière asynchrone (Cache L1 immédiat, puis Write-Behind L2/L3).
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée (Token JWT obligatoire).
// @Description  - Required permissions or roles: L'utilisateur doit être l'auteur strict du commentaire.
// @Description  - Relevant middleware: HMAC Validator, RateLimiter, JWT Auth.
// @Description  - Security checks: Le service `security_service.LeftComment` localise le commentaire. Si le commentaire n'existe pas ou appartient à autrui, la requête échoue avec 404 ou 403.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `comment_id` (int64), `content` (string, le nouveau texte).
// @Description  - Validation rules: Binding JSON. Le contenu est nettoyé via `pkg.CleanStr`. Le backend bloque formellement la mise à jour si le contenu nettoyé est vide (`EMPTY_COMMENT`) ou s'il dépasse physiquement la limite de 2200 caractères (`COMMENT_TOO_LONG`).
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Validation Synchrone** :
// @Description     - Extraction de l'ID utilisateur via middleware JWT (`pkg.GetUserIDFromContext`).
// @Description     - Binding du payload JSON.
// @Description     - Nettoyage du contenu et comptage précis des runes. Rejet immédiat si les limites ne sont pas respectées (0 ou >2200).
// @Description  2. **Vérification de Sécurité (Service)** :
// @Description     - `security_service.LeftComment` vérifie que le commentaire existe et appartient bien à l'utilisateur.
// @Description  3. **Application de la Modification** :
// @Description     - Le payload récupéré est muté en RAM (nouveau contenu, nouveau Timestamp `UpdatedAt`).
// @Description  4. **Persistance Hybride (Cache & Write-Behind)** :
// @Description     - Écrasement synchrone du commentaire dans l'Object Cache (L1) avec la nouvelle version.
// @Description     - Envoi de l'objet complet (ActionUpdate) dans la file d'attente asynchrone Redis pour les workers Batch (MongoDB / PostgreSQL).
// @Description  5. **Réponse Immédiate** : Renvoi du statut 200 OK.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: `{"message": "Commentaire mis à jour avec succès"}`
// @Description  - Persistence guarantees: L1 synchrone, L2/L3 asynchrones via Redis EnqueueDB.
// @Description  - Side effects: Aucune modification sur le post parent ou son score. Seul le texte et la date de modification du commentaire changent.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description  - **[INVALID_PAYLOAD] Format invalide:**
// @Description    - Trigger: Le format JSON est invalide ou des champs obligatoires sont manquants.
// @Description    - Execution stage: Validation GIN.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInvalidPayload` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description  - **[EMPTY_COMMENT] Commentaire vide:**
// @Description    - Trigger: Le texte est absent ou réduit à des espaces.
// @Description    - Execution stage: Validation des runes (Handler).
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: "EMPTY_COMMENT" (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description  - **[COMMENT_TOO_LONG] Commentaire trop long:**
// @Description    - Trigger: Le texte contient plus de 2200 caractères (runes).
// @Description    - Execution stage: Validation des runes (Handler).
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: "COMMENT_TOO_LONG" (returned in the `code` field of `numan_error.PublicErrorResponse`).
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
// @Description    - Trigger: Le commentaire existe mais n'appartient pas à l'utilisateur courant.
// @Description    - Execution stage: Vérification de sécurité (`LeftComment`).
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeForbidden` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  ⚫ **404 Not Found:**
// @Description  - **[RESOURCE_NOT_FOUND] Commentaire introuvable:**
// @Description    - Trigger: L'ID fourni ne correspond à aucun commentaire en base.
// @Description    - Execution stage: Vérification de sécurité (`LeftComment`).
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeNotFound` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description  - **[INTERNAL_SERVER_ERROR] Erreur serveur inattendue:**
// @Description    - Trigger: Panne de base de données ou échec d'insertion dans la file Redis (EnqueueDB).
// @Description    - Execution stage: Cache Write-Behind L2/L3.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInternalError` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Tags         comments
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        data          body   comment_models.UpdateCommentInput true "Nouveau contenu du commentaire"
// @Success      200  {object}  map[string]string "message: Commentaire mis à jour avec succès"
// @Failure      400  {object}  numan_error.PublicErrorResponse "Données invalides ou abus de caractères"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Session expirée ou utilisateur non identifié"
// @Failure      403  {object}  numan_error.PublicErrorResponse "Violation des droits d'auteur"
// @Failure      404  {object}  numan_error.PublicErrorResponse "Commentaire introuvable"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Erreur interne du serveur"
// @Router       /comment/update [put]
func UpdateCommentHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input comment_models.UpdateCommentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou champs manquants.", err))
		return
	}

	err = comment_service.UpdateComment(c.Request.Context(), callerID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Commentaire mis à jour avec succès"})
}
