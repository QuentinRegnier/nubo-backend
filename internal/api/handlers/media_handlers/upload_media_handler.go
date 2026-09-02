package media_handlers

import (
	"mime/multipart"
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/gin-gonic/gin"
)

// UploadMediaHandler godoc
// @Summary      Uploader un média (Out-of-Band)
// @Description  Traite l'image (AVIF, Resize), l'envoie sur le S3 et crée une coquille en attente (Visibility: false) en BDD.
// @Tags         media
// @Accept       multipart/form-data
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        file          formData file   true "Fichier image à uploader"
// @Success      200  {object} media_models.UploadMediaOutput "L'ID du média généré"
// @Failure      400  {object} nubo_error.PublicErrorResponse "Fichier manquant ou invalide"
// @Failure      401  {object} nubo_error.PublicErrorResponse "Non autorisé"
// @Failure      500  {object} nubo_error.PublicErrorResponse "Erreur interne de traitement"
// @Router       /media/upload [post]
func UploadMediaHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("MISSING_FILE", "Fichier manquant ou invalide.", err))
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("FILE_READ_ERROR", "Impossible de lire le fichier.", err))
		return
	}
	defer func(file multipart.File) {
		err := file.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur lors de la fermeture du fichier uploadé")
		}
	}(file)

	mediaID := pkg.GenerateID()
	if err := media_service.UploadMedia(file, callerID, mediaID, false); err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, media_models.UploadMediaOutput{
		MediaID: mediaID,
	})
}
