package post_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/service/post_service"
	"github.com/gin-gonic/gin"

	"github.com/QuentinRegnier/numan-backend/internal/pkg"
)

// UpdatePostHandler godoc
// @Summary      Mettre à jour une publication
// @Description  Applique des modifications au contenu et métadonnées d'un post (texte, hashtags, emplacement) avec re-vectorisation IA immédiate, écrasement du cache RAM et asynchronisme BDD.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée nécessitant un token JWT d'authentification valide.
// @Description  - Required permissions or roles: L'utilisateur appelant doit posséder le post ciblé.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Extraction et Normalisation :**
// @Description     - Extraction de l'appelant via le contexte HTTP.
// @Description     - Binding GIN du modèle `UpdatePostInput`.
// @Description     - Nettoyage rigoureux : déduplication des hashtags, `pkg.CleanStr` sur le texte brut et la localisation.
// @Description  2. **Sécurité Zero-Trust et Chargement L1 :**
// @Description     - Utilisation stricte de `security_service.LeftPost` pour charger l'état existant du post et vérifier que l'appelant en est l'auteur.
// @Description  3. **Modification et Algorithmique Embarquée :**
// @Description     - Mutation des attributs en mémoire et incrémentation de la variable de suivi d'IA (`VectorVersion + 1`).
// @Description     - Exécution synchrone locale du recalcul des affinités sémantiques (`ComputeContentVectorFull`).
// @Description  4. **Mise à jour Multi-Niveaux :**
// @Description     - Écrasement immédiat de l'objet LFU en mémoire RAM (Object Cache).
// @Description     - Soumission du post entier modifié à `redis.EnqueueDB` pour forcer un Bulk Update asynchrone par les workers backend.
// @Description  5. **Réponse HTTP :**
// @Description     - Renvoi d'un statut 200 avec message de succès.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description
// @Description  - **[numan_error.CodeInvalidPayload] Requête mal formatée ou paramètres invalides:**
// @Description    - Trigger: Format JSON invalide.
// @Description    - Execution stage: Validation GIN.
// @Description    - Response: `numan_error.PublicErrorResponse` avec détails, ou map JSON selon le format du routeur HTTP.
// @Description    - Error code: `numan_error.CodeInvalidPayload`
// @Description
// @Description  🟠 **403 Forbidden:**
// @Description
// @Description  - **[numan_error.CodeForbidden] Accès refusé:**
// @Description    - Trigger: L'appelant tente de mettre à jour un post qu'il n'a pas rédigé.
// @Description    - Execution stage: Évaluation Zero-Trust (`security_service.LeftPost`).
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeForbidden`
// @Description
// @Description  🟤 **404 Not Found:**
// @Description
// @Description  - **[numan_error.CodeNotFound] Ressource introuvable:**
// @Description    - Trigger: Le post ciblé n'existe pas ou a été effacé.
// @Description    - Execution stage: Évaluation Zero-Trust (`security_service.LeftPost`).
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeNotFound`
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description
// @Description  - **[numan_error.CodeInternalError] Erreur de service inattendue:**
// @Description    - Trigger: Défaut d'accès au cache objet interdisant la sécurité de l'appel, ou échec de l'enfilement Redis.
// @Description    - Execution stage: Exécution profonde du service métier.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInternalError`
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   post_models.UpdatePostInput true "Payload pour mettre à jour un post spécifique (post_id requis)"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      403  {object}  numan_error.PublicErrorResponse "Forbidden"
// @Failure      404  {object}  numan_error.PublicErrorResponse "Not Found"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /posts/update [put]
func UpdatePostHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input post_models.UpdatePostInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou validation échouée.", err))
		return
	}

	err = post_service.UpdatePost(c.Request.Context(), callerID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Post mis à jour avec succès"})
}
