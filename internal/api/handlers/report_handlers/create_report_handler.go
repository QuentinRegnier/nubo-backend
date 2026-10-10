package report_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/report_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/report_service"
	"github.com/gin-gonic/gin"
)

// CreateReportHandler godoc
// @Summary      Créer un signalement
// @Description  Permet à l'utilisateur courant de signaler un contenu (Post, Commentaire, Profil, etc.).
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: L'utilisateur doit être authentifié (Token valide).
// @Description  - Required permissions or roles: Membre standard.
// @Description  - Relevant middleware: ExtractUserID.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `TargetType`, `TargetIDs`, `Category`, `Reason` via le modèle `report_models.CreateReportInput`.
// @Description  - Validation rules: L'ID de l'utilisateur qui signale est extrait de manière sécurisée depuis le contexte GIN.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Extraction de l'appelant** : Récupération du `UserID` via `pkg.GetUserIDFromContext`.
// @Description  2. **Validation GIN** : Binding JSON vers `CreateReportInput` et validation basique.
// @Description  3. **Appel au service** : Transmission au service métier `SubmitReport`.
// @Description  4. **Évaluation d'urgence** : Calcul synchrone du score d'importance économique (O(1) en RAM) via `calculateEconomicImportance`.
// @Description  5. **Construction du Payload** : Création de l'objet `ReportPayload` (statut Pending).
// @Description  6. **Persistance Asynchrone** : Envoi du rapport en file d'attente (Write-Behind Redis via `EnqueueDB`) avec pour cible `TargetPostgres`.
// @Description  7. **Réponse** : Retour d'un message générique rassurant le client.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: Message de confirmation JSON ("Votre signalement a été pris en compte...").
// @Description  - Persistence guarantees: Mise en file d'attente Redis (asynchrone vers Postgres).
// @Description  - Side effects: Création d'une tâche de persistance DB.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description
// @Description  - **[INVALID_PAYLOAD] Invalid format or validation failed:**
// @Description    - Trigger: Le payload JSON est mal formé, ou les champs (catégorie, type cible) manquent.
// @Description    - Execution stage: Validation GIN `ShouldBindJSON`.
// @Description    - Response: `numan_error.PublicErrorResponse` ("Données invalides...").
// @Description    - Error code: `INVALID_PAYLOAD` (ou `numan_error.CodeInvalidPayload`).
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description
// @Description  - **[INTERNAL_ERROR] Échec de la file d'attente:**
// @Description    - Trigger: Échec de `redis.EnqueueDB` lors de l'enregistrement asynchrone du signalement.
// @Description    - Execution stage: `SubmitReport` (Étape 3 : Persistance Asynchrone).
// @Description    - Response: `numan_error.PublicErrorResponse` générique.
// @Description    - Error code: `numan_error.CodeInternal`.
// @Tags         report
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   report_models.CreateReportInput true "Payload pour créer un signalement (type cible, IDs, catégorie, raison)"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /report [post]
func CreateReportHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input report_models.CreateReportInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	if err := report_service.SubmitReport(c.Request.Context(), callerID, input); err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Votre signalement a été pris en compte et sera étudié par nos équipes."})
}
