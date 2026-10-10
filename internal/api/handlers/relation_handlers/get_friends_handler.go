package relation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/relation_service"
	"github.com/gin-gonic/gin"
)

// GetFriendsHandler godoc
// @Summary      Récupérer la liste des amis
// @Description  Renvoie la liste des utilisateurs qui sont amis avec la cible spécifiée (état de relation mutuelle "friend").
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route authentifiée.
// @Description  - Required permissions or roles: Vérification contextuelle.
// @Description  - Relevant middleware: ExtractUserID.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `TargetID`. Pagination optionnelle `Limit`, `Offset`.
// @Description  - Validation rules: Binding GIN standard sur `relation_models.GetFriendsInput`.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Validation GIN** : Parsing du payload JSON depuis le client.
// @Description  2. **Nettoyage Limites** : Si `Limit` vaut 0, forcer une limite par défaut de 50 pour éviter les surcharges de requêtes.
// @Description  3. **Appel au service métier** : `GetFriends` exécuté pour le Caller et la Cible.
// @Description  4. **Requête DB/Cache** : Utilisation de `fetchRelationsHydrated` en mode "incoming" avec la constante d'état "Friend" (2).
// @Description  5. **Résolution des profils** : Les ID des amis sont convertis en profils complets (ou vues partielles) depuis le système de cache/base.
// @Description  6. **Réponse HTTP** : Constitution de l'objet final `GetFriendsOutput`.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: `relation_models.GetFriendsOutput` contenant la liste `Users`.
// @Description  - Persistence guarantees: Aucune écriture (opération de lecture seule).
// @Description  - Side effects: Aucun effet de bord persistant.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description
// @Description  - **[INVALID_JSON] Invalid format or validation failed:**
// @Description    - Trigger: Les paramètres fournis ne sont pas du JSON valide ou manquent les types stricts.
// @Description    - Execution stage: `c.ShouldBindJSON`.
// @Description    - Response: `numan_error.PublicErrorResponse` ("Corps de requête invalide.").
// @Description    - Error code: `INVALID_JSON` (ou `numan_error.CodeInvalidPayload`).
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description
// @Description  - **[INTERNAL_ERROR] Erreur serveur lors de la récupération des amis:**
// @Description    - Trigger: Le processus d'hydratation échoue (timeout Redis, DB down, etc.).
// @Description    - Execution stage: Traitement des données par `fetchRelationsHydrated`.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInternal`.
// @Tags         relations
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   relation_models.GetFriendsInput true "Payload pour récupérer la liste des amis d'une cible"
// @Success      200  {object}  relation_models.GetFriendsOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /friend/get [post]
func GetFriendsHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input relation_models.GetFriendsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Invalid JSON ou validation échouée.", err))
		return
	}

	// Cible à remplacer par la vraie variable dans ton scope
	output, errOut := relation_service.GetFriends(c.Request.Context(), callerID, input)
	if errOut != nil {
		numan_error.RespondWithError(c, errOut)
		return
	}

	c.JSON(http.StatusOK, output)
}
