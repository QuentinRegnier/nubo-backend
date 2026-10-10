package search_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/search_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/search_service"
	"github.com/gin-gonic/gin"
)

// AutocompleteTextHandler godoc
// @Summary      Autocomplétion globale (Utilisateurs & Communautés)
// @Description  Moteur d'autocomplétion textuelle interrogeant en parallèle plusieurs domaines rapides (sans recherche textuelle complète de posts).
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée.
// @Description  - Relevant middleware: ExtractUserID.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `Query`, `Limit` (optionnel) via `search_models.AutocompleteTextInput`.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Extraction de l'appelant** : Récupération du CallerID via `GetUserIDFromContext`.
// @Description  2. **Validation GIN** : Binding JSON des données d'entrée.
// @Description  3. **Valeur par Défaut** : Application de la `DefaultGlobalSearchLimit` si `Limit` est absent ou 0.
// @Description  4. **Parallélisme Métier** : Exécution de `searchUsers` et `searchCommunities` avec traitement tolérant aux pannes :
// @Description     - **Utilisateurs** : Recherche lexicographique (ZSET Lex) suivie d'une hydratation des avatars à la volée (`GenerateMediaViewCascade`).
// @Description     - **Communautés** : Recherche similaire sur l'index des communautés.
// @Description     *Note : Une erreur sur l'un des services n'entraîne pas un plantage de l'autocomplétion.*
// @Description  5. **Assemblage** : Structuration asynchrone ou séquentielle (selon le service) des résultats.
// @Description  6. **Réponse** : Renvoi de `AutocompleteTextOutput`.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: `search_models.AutocompleteTextOutput` contenant `Users` et `Communities`.
// @Description  - Persistence guarantees: 100% Cache L1 (RAM et ZSET Lex) prioritaire pour `searchUsers`.
// @Description  - Side effects: Génération d'URLs Média signées (Lecture seule).
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description
// @Description  - **[INVALID_PAYLOAD] Invalid format:**
// @Description    - Trigger: Le body JSON est invalide ou absent.
// @Description    - Execution stage: Validation GIN.
// @Description    - Response: `numan_error.PublicErrorResponse` ("Corps de requête invalide.").
// @Description    - Error code: `INVALID_PAYLOAD`.
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description
// @Description  - **[INTERNAL_ERROR] Échec fatal global:**
// @Description    - Trigger: Défaillance inattendue ou critique dans la cascade d'autocomplétion non capturée par les isolements.
// @Description    - Execution stage: Couche Service (rare car les erreurs unitaires sont ignorées pour renvoyer des listes vides).
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInternal`.
// @Tags         search
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   search_models.AutocompleteTextInput true "Payload pour autocomplétion globale (préfixe et limite)"
// @Success      200  {object}  search_models.AutocompleteTextOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /search/autocomplete/text [post]
func AutocompleteTextHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input search_models.AutocompleteTextInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	output, err := search_service.AutocompleteText(c.Request.Context(), callerID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, output)
}
