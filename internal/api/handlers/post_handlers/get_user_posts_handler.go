package post_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/gin-gonic/gin"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/post_service"
)

// GetUserPostsHandler godoc
// @Summary      Récupérer la timeline d'un profil
// @Description  Renvoie la chronologie des posts d'un utilisateur cible avec un mécanisme de récupération L1/L3 optimisé et un fallback automatique d'auto-guérison du cache L1.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée nécessitant un token JWT d'authentification valide.
// @Description  - Required permissions or roles: L'appelant doit être identifié, la visibilité effective sera évaluée lors de l'hydratation des posts.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Validation et Préparation :**
// @Description     - Extraction de l'appelant via le contexte HTTP.
// @Description     - Binding GIN du payload (avec initialisation automatique de la limite à 50 si non fournie ou hors bornes).
// @Description  2. **Exploration de la Timeline ZSET (L1) :**
// @Description     - Interrogation prioritaire du Cache L1 (`cache_service.GetTopUserPostIDs`) sauf contournement forcé.
// @Description     - Retour immédiat certifié vide si le profil est marqué sans contenu.
// @Description  3. **Mécanisme de Fallback (L3 Postgres) et Auto-Guérison :**
// @Description     - Si Cache Miss, lecture structurée directement sur PostgreSQL.
// @Description     - Reconstruction synchrone du ZSET L1 en mémoire avec les posts récents pour protéger les appels suivants.
// @Description     - Injection de protection anti-fantôme si la BDD est réellement vide.
// @Description  4. **Délégation d'Hydratation :**
// @Description     - Délégation des IDs récoltés à `post_service.GetPosts` (Garantissant ainsi un Hit Cache Objet L1 à 100% sur les payloads et l'application des règles relationnelles).
// @Description  5. **Réponse :**
// @Description     - Renvoi de la liste finale `[]GetPostOutput` en statut 200 OK.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description
// @Description  - **[numan_error.CodeInvalidPayload] Requête mal formatée ou paramètres invalides:**
// @Description    - Trigger: Format JSON invalide ou identifiant d'utilisateur (`user_id`) manquant.
// @Description    - Execution stage: Validation GIN.
// @Description    - Response: `numan_error.PublicErrorResponse` avec détails, ou map JSON selon le format du routeur HTTP.
// @Description    - Error code: `numan_error.CodeInvalidPayload`
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description
// @Description  - **[numan_error.CodeInternalError] Erreur de service inattendue:**
// @Description    - Trigger: Erreur critique de la base de données principale rendant la création du fallback impossible ou le service inopérant.
// @Description    - Execution stage: Exécution profonde du service métier.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInternalError`
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   post_models.GetUserPostsInput true "Payload pour récupérer les posts d'un utilisateur"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /posts/user/get [post]
func GetUserPostsHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input post_models.GetUserPostsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou user_id manquant.", err))
		return
	}

	posts, err := post_service.GetUserPosts(c.Request.Context(), callerID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, posts)
}
