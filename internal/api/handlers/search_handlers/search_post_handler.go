package search_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/search_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/search_service"
	"github.com/gin-gonic/gin"
)

// SearchPostHandler godoc
// @Summary      Moteur de recherche complexe de publications
// @Description  Permet d'effectuer des requêtes Full-Text, par hashtags exacts, par auteurs, et de trier avec les algorithmes du domaine Post.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée.
// @Description  - Relevant middleware: ExtractUserID.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `Query` (optionnel si filtrage strict), `Filter` (tri/rang), `Limit`, `Offset` dans `search_models.SearchPostInput`.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Validation & Context** : Extraction du JSON et récupération de l'appelant.
// @Description  2. **Mapping du Mode de Tri** : Convertit le champ optionnel `Filter` en une constante interne `OrderMode` (Recent, Likes, Views, etc.). Par défaut : `OrderModeRecent`.
// @Description  3. **Routage de la Stratégie (SearchPosts)** :
// @Description     - *Cas 1 (Filtres Vides)* : Si `Query` est vide et que `Filter` est global (ex: strict:likes), lecture directe depuis les Rankings en Cache L1.
// @Description     - *Cas 2 (Hashtag)* : Si `Query` commence par `#`, interception vers le ZSET L1 (si récent) ou PostgreSQL (si triage demandé).
// @Description     - *Cas 3 (Auteur)* : Si `Query` commence par `@`, résolution O(log N) de l'ID utilisateur, puis lecture Postgres L3.
// @Description     - *Cas 4 (Full-Text Fallback)* : Interrogation complète de l'index textuel Postgres (`FuncSearchPostIDsByText`).
// @Description  4. **Coupe-circuit** : Si aucun ID n'est trouvé, la recherche s'arrête net (renvoie array vide).
// @Description  5. **Hydratation Unifiée** : Appel direct du pipeline `post_service.GetPosts` avec la liste finale d'IDs pour récupérer les documents complexes (privilèges, S3 presign, compteurs, commentaires).
// @Description  6. **Réponse** : Renvoi des posts hydratés et d'un flag booléen (`HasTrendFilterAvailable`).
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: `search_models.SearchPostOutput` comprenant la collection de Posts complets.
// @Description  - Persistence guarantees: L1 Redis pour le temps-réel, PostgreSQL (pg_trgm / tsvector) pour les cas Full-Text.
// @Description  - Side effects: Aucun enregistrement métier (Lecture seule pure).
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description
// @Description  - **[INVALID_PAYLOAD] Invalid format:**
// @Description    - Trigger: Le JSON est mal structuré ou inadapté à la structure.
// @Description    - Execution stage: Validation GIN `ShouldBindJSON`.
// @Description    - Response: `numan_error.PublicErrorResponse` ("Corps de requête invalide.").
// @Description    - Error code: `INVALID_PAYLOAD`.
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description
// @Description  - **[INTERNAL_ERROR] Échec L3 Critique:**
// @Description    - Trigger: La base de données PostgreSQL ne répond pas ou la syntaxe Full-Text provoque une panique interne.
// @Description    - Execution stage: Étape de fallback PostgreSQL (`FuncSearchPostIDsByText`).
// @Description    - Response: `numan_error.PublicErrorResponse` générique.
// @Description    - Error code: `numan_error.CodeInternal`.
// @Tags         search
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   search_models.SearchPostInput true "Payload pour recherche de publications (texte, hashtags, auteurs, filtres, pagination)"
// @Success      200  {object}  search_models.SearchPostOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /search/post [post]
func SearchPostHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input search_models.SearchPostInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	output, err := search_service.SearchPosts(c.Request.Context(), callerID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, output)
}
