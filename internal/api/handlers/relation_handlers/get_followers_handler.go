package relation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/relation_service"
	"github.com/gin-gonic/gin"
)

// GetFollowersHandler godoc
// @Summary      Récupérer la liste des abonnés (Followers)
// @Description  Renvoie la liste des utilisateurs qui s'abonnent à la cible spécifiée (relations entrantes "incoming").
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée nécessitant un Token valide.
// @Description  - Required permissions or roles: Vérification contextuelle classique.
// @Description  - Relevant middleware: ExtractUserID.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `TargetID` (la cible dont on veut voir les followers). `Limit`, `Offset`.
// @Description  - Validation rules: Binding GIN standard sur `relation_models.GetFollowersInput`.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Validation GIN** : Binding JSON vers `GetFollowersInput`.
// @Description  2. **Valeurs par défaut** : Application de la limite 50 si aucune n'est passée.
// @Description  3. **Appel au Service métier** : Exécution de `GetFollows` dans le scope de la cible.
// @Description  4. **Extraction Hydratée** : Interrogation de `fetchRelationsHydrated` avec "incoming" sur l'état "Follow" (1).
// @Description  5. **Récupération des Profils** : Lecture des données utilisateurs et vérification de la réciprocité en cache.
// @Description  6. **Restitution** : Envoi de `relation_models.GetFollowersOutput`.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: `relation_models.GetFollowersOutput` contenant le tableau des followers.
// @Description  - Persistence guarantees: Récupération en mémoire (redis/RAM) prioritairement, sinon base de données.
// @Description  - Side effects: Aucun.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description
// @Description  - **[INVALID_JSON] Erreur de parsing du payload:**
// @Description    - Trigger: Le corps JSON est défectueux ou ne correspond pas au modèle de struct.
// @Description    - Execution stage: Validation GIN.
// @Description    - Response: `numan_error.PublicErrorResponse` ("Corps de requête invalide.").
// @Description    - Error code: `INVALID_JSON` (ou `numan_error.CodeInvalidPayload`).
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description
// @Description  - **[INTERNAL_ERROR] Échec de lecture ou d'hydratation:**
// @Description    - Trigger: Panne de base de données, timeout lors de la récupération des données utilisateur complètes.
// @Description    - Execution stage: `fetchRelationsHydrated`.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInternal`.
// @Tags         relations
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   relation_models.GetFollowersInput true "Payload pour récupérer la liste des abonnés d'une cible"
// @Success      200  {object}  relation_models.GetFollowersOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /follow/get [post]
func GetFollowersHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input relation_models.GetFollowersInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Invalid JSON ou validation échouée.", err))
		return
	}

	// Cible à remplacer par la vraie variable dans ton scope
	output, errOut := relation_service.GetFollows(c.Request.Context(), callerID, input)
	if errOut != nil {
		numan_error.RespondWithError(c, errOut)
		return
	}

	c.JSON(http.StatusOK, output)
}
