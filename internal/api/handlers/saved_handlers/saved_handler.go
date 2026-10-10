package saved_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/saved_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/saved_service"
	"github.com/gin-gonic/gin"
)

// SavePostHandler godoc
// @Summary      Sauvegarder un post
// @Description  Ajoute un post à la liste des favoris de l'utilisateur.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée.
// @Description  - Relevant middleware: ExtractUserID.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `PostID` dans le body JSON (`saved_models.SaveActionInput`).
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Extraction de l'appelant** : Récupération du `UserID` via `pkg.GetUserIDFromContext`.
// @Description  2. **Validation GIN** : Binding JSON du PostID (`ShouldBindJSON`).
// @Description  3. **Intégrité (Cache Object)** : Vérification en L1 de l'existence du Post. Si absent, renvoie NotFound 404 (pour éviter la sauvegarde d'un fantôme).
// @Description  4. **Traitement L1** : Ajout du PostID dans le Redis ZSET de l'utilisateur (Score = Timestamp).
// @Description  5. **Persistance Asynchrone** : EnqueueDB de la sauvegarde (ActionCreate) via PartitionKey sur le UserID (Write-Behind Redis).
// @Description  6. **Réponse** : Retourne 200 OK avec message générique.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: Message de confirmation générique.
// @Description  - Persistence guarantees: Mise à jour immédiate ZSET, différée DB.
// @Description  - Side effects: Tâche asynchrone générée.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description
// @Description  - **[INVALID_PAYLOAD] Invalid format:**
// @Description    - Trigger: Le body est manquant ou ne contient pas l'entier `PostID`.
// @Description    - Execution stage: Validation GIN.
// @Description    - Response: `numan_error.PublicErrorResponse` ("Format JSON invalide...").
// @Description    - Error code: `INVALID_PAYLOAD`.
// @Description
// @Description  🟠 **404 Not Found:**
// @Description
// @Description  - **[NOT_FOUND] Publication introuvable:**
// @Description    - Trigger: Le `PostID` ciblé n'existe pas dans l'Object Cache.
// @Description    - Execution stage: Étape 1 du `ToggleSaved` (Intégrité du Post).
// @Description    - Response: `numan_error.PublicErrorResponse` ("La publication est introuvable ou indisponible.").
// @Description    - Error code: `numan_error.CodeNotFound`.
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description
// @Description  - **[INTERNAL_ERROR] Échec file d'attente:**
// @Description    - Trigger: Redis EnqueueDB échoue.
// @Description    - Execution stage: Persistance asynchrone (`ToggleSaved`).
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInternal`.
// @Tags         saved
// @Accept       json
// @Produce      json
// @Param        Authorization header string false "Bearer <Current_JWT> (Optional)"
// @Param        X-Signature   header string true  "HMAC calculated with OLD MasterToken"
// @Param        X-Timestamp   header string true  "Unix Timestamp"
// @Param        input body saved_models.SaveActionInput true "Payload contenant le PostID"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      404  {object}  numan_error.PublicErrorResponse "Not Found"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /saved/set [post]
func SavePostHandler(c *gin.Context) {
	handleSavedAction(c, "save")
}

// UnsavePostHandler godoc
// @Summary      Retirer un post des sauvegardes
// @Description  Retire un post de la liste des favoris de l'utilisateur.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée.
// @Description  - Relevant middleware: ExtractUserID.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `PostID` dans le body JSON (`saved_models.SaveActionInput`).
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Extraction de l'appelant** : Récupération du `UserID` via `pkg.GetUserIDFromContext`.
// @Description  2. **Validation GIN** : Binding JSON du PostID (`ShouldBindJSON`).
// @Description  3. **Intégrité (Cache Object)** : Vérification de l'existence globale du Post en Cache (NotFound si inexistant).
// @Description  4. **Traitement L1** : Suppression du PostID depuis le Redis ZSET de l'utilisateur.
// @Description  5. **Persistance Asynchrone** : EnqueueDB de la suppression (ActionDelete) via PartitionKey sur le UserID (Write-Behind Redis).
// @Description  6. **Réponse** : Retourne 200 OK avec message générique.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: Message de confirmation générique.
// @Description  - Persistence guarantees: Mise à jour immédiate ZSET, différée DB.
// @Description  - Side effects: Tâche asynchrone générée (Suppression DB).
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description
// @Description  - **[INVALID_PAYLOAD] Invalid format:**
// @Description    - Trigger: Le body est manquant ou non conforme au JSON.
// @Description    - Execution stage: Validation GIN.
// @Description    - Response: `numan_error.PublicErrorResponse` ("Format JSON invalide...").
// @Description    - Error code: `INVALID_PAYLOAD`.
// @Description
// @Description  🟠 **404 Not Found:**
// @Description
// @Description  - **[NOT_FOUND] Publication introuvable:**
// @Description    - Trigger: Le `PostID` n'existe plus dans le cache applicatif.
// @Description    - Execution stage: Contrôle d'intégrité initial.
// @Description    - Response: `numan_error.PublicErrorResponse` ("La publication est introuvable ou indisponible.").
// @Description    - Error code: `numan_error.CodeNotFound`.
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description
// @Description  - **[INTERNAL_ERROR] Échec file d'attente:**
// @Description    - Trigger: Impossible de joindre la file d'attente Write-Behind.
// @Description    - Execution stage: Persistance asynchrone.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInternal`.
// @Tags         saved
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   saved_models.SaveActionInput true "Payload contenant le PostID"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      404  {object}  numan_error.PublicErrorResponse "Not Found"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /saved/delete [delete]
func UnsavePostHandler(c *gin.Context) {
	handleSavedAction(c, "unsave")
}

func handleSavedAction(c *gin.Context, action string) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input saved_models.SaveActionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	if err := saved_service.ToggleSaved(c.Request.Context(), callerID, input.PostID, action); err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Action '" + action + "' exécutée avec succès"})
}
