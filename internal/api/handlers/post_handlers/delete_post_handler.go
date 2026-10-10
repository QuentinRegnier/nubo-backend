package post_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/gin-gonic/gin"

	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/post_service"
)

// DeletePostHandler godoc
// @Summary      Supprimer une publication
// @Description  Effectue la suppression sécurisée d'un post. La suppression purgera instantanément tous les caches L1 (Post, Timeline, Commentaires associés, Vecteurs IA, Médias) et déclenchera la suppression asynchrone BDD.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée nécessitant un token JWT d'authentification valide.
// @Description  - Required permissions or roles: L'utilisateur appelant doit être identifié et doit obligatoirement être le créateur (propriétaire) du post ciblé.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Extraction et validation des entrées :**
// @Description     - Extraction de l'ID appelant via le contexte HTTP.
// @Description     - Binding GIN pour extraire le `PostID` à supprimer.
// @Description  2. **Contrôles de Sécurité Zero-Trust :**
// @Description     - Appel de `security_service.LeftPost` pour charger le post ciblé et vérifier rigoureusement l'appartenance à l'utilisateur appelant.
// @Description  3. **Purge Massive Synchrone (RAM L1) :**
// @Description     - Destruction physique du payload en cache objet.
// @Description     - Purge depuis la timeline utilisateur (`cache_service.RemovePostFromUserProfile`).
// @Description     - Purge des commentaires en cache associés au post.
// @Description     - Suppression des vecteurs algorithmiques (LSH).
// @Description     - Suppression des vues médias L1.
// @Description  4. **Persistance Asynchrone et Réponse :**
// @Description     - Insertion dans la file `redis.EnqueueDB` avec l'instruction `ActionDelete` pour synchroniser les bases de données froides.
// @Description     - Réponse HTTP 200 OK avec message de confirmation.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description
// @Description  - **[numan_error.CodeInvalidPayload] Requête mal formatée ou paramètres invalides:**
// @Description    - Trigger: Format JSON invalide ou ID de post manquant.
// @Description    - Execution stage: Validation GIN.
// @Description    - Response: `numan_error.PublicErrorResponse` avec détails, ou map JSON selon le format du routeur HTTP.
// @Description    - Error code: `numan_error.CodeInvalidPayload`
// @Description
// @Description  🟠 **403 Forbidden:**
// @Description
// @Description  - **[numan_error.CodeForbidden] Accès refusé:**
// @Description    - Trigger: L'utilisateur appelant tente de supprimer un post dont il n'est pas l'auteur.
// @Description    - Execution stage: Évaluation Zero-Trust par `security_service.LeftPost`.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeForbidden`
// @Description
// @Description  🟤 **404 Not Found:**
// @Description
// @Description  - **[numan_error.CodeNotFound] Ressource introuvable:**
// @Description    - Trigger: Le post ciblé n'existe pas ou a déjà été supprimé.
// @Description    - Execution stage: Tentative de chargement sécurisé via `security_service.LeftPost`.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeNotFound`
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description
// @Description  - **[numan_error.CodeInternalError] Erreur de service inattendue:**
// @Description    - Trigger: Échec critique lors du placement en file d'attente Redis pour suppression persistante.
// @Description    - Execution stage: Exécution profonde du service métier.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInternalError`
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   post_models.DeletePostInput true "Payload pour supprimer un post spécifique (post_id requis)"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      403  {object}  numan_error.PublicErrorResponse "Forbidden"
// @Failure      404  {object}  numan_error.PublicErrorResponse "Not Found"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /posts/delete [delete]
func DeletePostHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input post_models.DeletePostInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou post_id manquant.", err))
		return
	}

	err = post_service.DeletePost(c.Request.Context(), callerID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Post supprimé avec succès"})
}
