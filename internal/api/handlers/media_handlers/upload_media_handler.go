package media_handlers

import (
	"fmt"
	"mime/multipart"
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
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
// @Failure      400  {object} nubo_error.ErrorResponse "Fichier manquant ou invalide"
// @Failure      401  {object} nubo_error.ErrorResponse "Non autorisé"
// @Failure      500  {object} nubo_error.ErrorResponse "Erreur interne de traitement"
// @Router       /media/upload [post]
func UploadMediaHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, nubo_error.ErrorResponse{Error: "Utilisateur non identifié"})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Fichier manquant ou invalide"})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, nubo_error.ErrorResponse{Error: "Impossible de lire le fichier"})
		return
	}
	defer func(file multipart.File) {
		err := file.Close()
		if err != nil {
			fmt.Println("Erreur lors de la fermeture du fichier:", err)
		}
	}(file)

	mediaID := pkg.GenerateID()
	if err := media_service.UploadMedia(file, callerID, mediaID, false); err != nil {
		c.JSON(http.StatusInternalServerError, nubo_error.ErrorResponse{Error: "Impossible de traiter et d'uploader le média: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, media_models.UploadMediaOutput{
		MediaID: mediaID,
	})
}
