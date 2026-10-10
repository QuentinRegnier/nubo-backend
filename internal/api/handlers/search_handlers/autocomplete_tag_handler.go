package search_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/search_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/search_service"
	"github.com/gin-gonic/gin"
)

// AutocompleteTagHandler godoc
// @Summary      Autocomplétion des hashtags
// @Description  Fournit une liste ultra-rapide de hashtags correspondant au préfixe saisi par l'utilisateur.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route publique ou sécurisée (selon middleware).
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `Query` (le préfixe), `Limit` (limite max des résultats) via `search_models.AutocompleteTagInput`.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Validation GIN** : Binding JSON et contrôle des entrées.
// @Description  2. **Nettoyage et Fallback** : Application de la limite par défaut si `Limit` = 0. Normalisation (Lower, TrimSpace) de `Query`.
// @Description  3. **Recherche Lexicographique (L1)** : Requête sur Redis ZSET en O(log N) pour obtenir les correspondances directes.
// @Description  4. **Magie Sémantique (Graphe de Markov)** : Si la requête correspond EXACTEMENT à un hashtag existant, interrogation d'un cache sémantique (`GetRelatedTagsLazy`) en O(1) RAM. Les tags sémantiquement liés sont triés par pertinence (Weight décroissant) et injectés juste après le tag exact.
// @Description  5. **Remplissage Fallback** : Complétion des résultats manquants avec les suggestions lexicographiques classiques non utilisées.
// @Description  6. **Réponse** : Assemblage d'un tableau de chaînes, avec garantie stricte anti-null (`[]`).
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: `search_models.AutocompleteTagOutput` avec le tableau `Tags` de strings.
// @Description  - Persistence guarantees: Lecture uniquement depuis les caches locaux/Redis (0 accès DB PostgreSQL).
// @Description  - Side effects: Aucun.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description
// @Description  - **[INVALID_PAYLOAD] Invalid format:**
// @Description    - Trigger: Le body est illisible ou types invalides.
// @Description    - Execution stage: Validation GIN (`ShouldBindJSON`).
// @Description    - Response: `numan_error.PublicErrorResponse` ("Corps de requête invalide.").
// @Description    - Error code: `INVALID_PAYLOAD`.
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description
// @Description  - **[INTERNAL_ERROR] Échec L1 Redis:**
// @Description    - Trigger: Redis ou le cache RAM est indisponible lors de la requête lexicographique.
// @Description    - Execution stage: `SearchTagsByPrefix` (Étape 3).
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInternal`.
// @Tags         search
// @Accept       json
// @Produce      json
// @Param        Authorization header string false "Bearer <Current_JWT> (Optional)"
// @Param        X-Signature   header string true  "HMAC calculated with OLD MasterToken"
// @Param        X-Timestamp   header string true  "Unix Timestamp"
// @Param        input         body   search_models.AutocompleteTagInput true "Payload pour autocomplétion des hashtags (préfixe et limite)"
// @Success      200  {object}  search_models.AutocompleteTagOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /search/autocomplete/tags [post]
func AutocompleteTagHandler(c *gin.Context) {
	_, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input search_models.AutocompleteTagInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	output, err := search_service.AutocompleteTags(c.Request.Context(), input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, output)
}
