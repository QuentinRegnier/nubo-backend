package media_handlers

import (
	"mime/multipart"
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_log"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/media_service"
	"github.com/gin-gonic/gin"
)

// UploadMediaHandler godoc
// @Summary      Téléverser un média
// @Description  Téléverse une image depuis un flux `multipart/form-data`, vérifie sa conformité (pixels, taille max), la décode, la redimensionne homothétiquement, l'encode en format AVIF optimisé, et l'uploade vers le stockage objet MinIO/S3.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée (Token JWT requis).
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: Un fichier image fourni via form-data sous le nom `file`.
// @Description  - Validation rules: Le fichier doit être lisible, sa résolution totale (Largeur x Hauteur) ne doit pas dépasser le `MediaMaxPixels` autorisé.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Extraction de la Requête** : Récupération du `callerID` et extraction du fichier binaire envoyé sous la clé form `file`.
// @Description  2. **Analyse Image & Dimensionnement** : Ouverture en flux pour décoder la config (`image.DecodeConfig`) et vérifier la limite de pixels (Rejet avec code `PAYLOAD_TOO_LARGE` si dépassée). Réinitialisation de la tête de lecture.
// @Description  3. **Décodage & Conversion AVIF (CPU Heavy)** : Décodage complet. Redimensionnement (via filtre de Lanczos) si la largeur excède `MediaMaxWidth`. Encodage final de la matrice en format `.avif`.
// @Description  4. **Upload S3 / MinIO** : Transfert synchrone du fichier compilé vers le bucket configuré sur un chemin unique rattaché à l'utilisateur `users/{ID}/media/{UUID}.avif`.
// @Description  5. **Mise en Base** : L'ID est généré, le fichier en mémoire RAM L1 est provisionné en Out-Of-Band (`Visibility: false`, c'est-à-dire attente d'association à un post/profil). Déclenchement de la persistance asynchrone (Write-Behind) Redis. Un mécanisme de Rollback supprime le fichier du S3 si l'enregistrement Redis échoue.
// @Description  6. **Réponse** : Retour au client du MediaID généré.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 202 Accepted
// @Description  - Response body: `media_models.UploadMediaOutput` contenant le nouvel identifiant `MediaID`.
// @Description  - Side effects: Création de fichier sur l'Object Storage, ajout du payload MediaPayload non visible au ZSET Redis/Cache.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request — Erreurs de Fichier ou Modèle**
// @Description  - **[MISSING_FILE] Fichier manquant:**
// @Description    - **Trigger:** Aucune donnée multipart trouvée pour le champ nommé `file`.
// @Description    - **Execution stage:** Extraction depuis le contexte GIN (`c.FormFile("file")`).
// @Description    - **Response:** Structure JSON `numan_error.PublicErrorResponse`.
// @Description    - **Error code:** `numan_error.CodeMissingFile` (Littéral exact renvoyé dans le champ JSON `code`).
// @Description  - **[FILE_READ_ERROR] Impossible de lire le fichier:**
// @Description    - **Trigger:** Le fichier a bien été transmis mais ne peut pas être ouvert.
// @Description    - **Execution stage:** Fonction `fileHeader.Open()`.
// @Description    - **Response:** Structure JSON `numan_error.PublicErrorResponse`.
// @Description    - **Error code:** `numan_error.CodeFileReadError` (Littéral exact renvoyé dans le champ JSON `code`).
// @Description  - **[INVALID_PAYLOAD] Fichier non image ou corrompu:**
// @Description    - **Trigger:** La configuration de l'image n'a pas pu être décodée, format non pris en charge ou corruption.
// @Description    - **Execution stage:** `image.DecodeConfig` dans le service `UploadMedia`.
// @Description    - **Response:** Structure JSON `numan_error.PublicErrorResponse`.
// @Description    - **Error code:** `numan_error.CodeInvalidPayload` (renvoyé dans le champ JSON `code`).
// @Description  - **[PAYLOAD_TOO_LARGE] Résolution excessive:**
// @Description    - **Trigger:** La taille en pixels excède la constante `variables.MediaMaxPixels`.
// @Description    - **Execution stage:** Après décodage config dans le service `UploadMedia`.
// @Description    - **Response:** Structure JSON `numan_error.PublicErrorResponse`.
// @Description    - **Error code:** `numan_error.CodePayloadTooLarge` (renvoyé dans le champ JSON `code`).
// @Description
// @Description  🟠 **401 Unauthorized — Authentification échouée**
// @Description  - **Trigger:** Le token JWT est absent, invalide ou expiré.
// @Description  - **Execution stage:** Middleware d'authentification ou `pkg.GetUserIDFromContext`.
// @Description  - **Response:** Structure JSON `numan_error.PublicErrorResponse`.
// @Description  - **Error code:** `numan_error.CodeUnauthorized` (renvoyé dans le champ JSON `code`).
// @Description
// @Description  ⚫ **500 Internal Server Error — Erreur inattendue (Décodage, S3, Base)**
// @Description  - **Trigger:** Impossible d'encoder l'AVIF, échec d'upload vers MinIO, échec de réinitialisation du flux, ou échec final d'EnqueueDB.
// @Description  - **Execution stage:** Processus `media_service.UploadMedia`.
// @Description  - **Response:** Structure JSON `numan_error.PublicErrorResponse`.
// @Description  - **Error code:** `numan_error.CodeInternalError` (renvoyé dans le champ JSON `code`).
// @Tags         media
// @Accept       multipart/form-data
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Success      202  {object}  media_models.UploadMediaOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Fichier manquant, Invalide ou Trop lourd"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Unauthorized"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Erreur Interne (Encodage, Minio, Redis)"
// @Router       /media/upload [post]
func UploadMediaHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeMissingFile, "Fichier manquant ou invalide.", err))
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeFileReadError, "Impossible de lire le fichier.", err))
		return
	}
	defer func(file multipart.File) {
		err := file.Close()
		if err != nil {
			numan_log.Error(c).Err(err).Msg("Erreur lors de la fermeture du fichier uploadé")
		}
	}(file)

	mediaID := pkg.GenerateID()
	if err := media_service.UploadMedia(file, callerID, mediaID, false); err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, media_models.UploadMediaOutput{
		MediaID: mediaID,
	})
}
