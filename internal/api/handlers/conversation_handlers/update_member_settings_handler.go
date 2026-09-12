package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// UpdateMemberSettingsHandler godoc
// @Summary      Mettre à jour les paramètres d'une conversation
// @Description  Permet de modifier les réglages personnels liés à une conversation (sourdine, ordre d'épinglage, etc.).
// @Description  Toutes les données (y compris l'ID de la conversation) doivent être passées dans le payload JSON.
// @Tags         conversations
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        payload body conversation_models.UpdateMemberSettingsInput true "Paramètres à mettre à jour"
// @Success      200 {object} map[string]string "Message de succès"
// @Failure      400 {object} nubo_error.PublicErrorResponse "JSON invalide ou conversation_id manquant"
// @Failure      401 {object} nubo_error.PublicErrorResponse "Non autorisé"
// @Failure      403 {object} nubo_error.PublicErrorResponse "L'utilisateur n'est pas membre de la conversation"
// @Failure      500 {object} nubo_error.PublicErrorResponse "Erreur interne"
// @Router       /conversation/settings [patch]
func UpdateMemberSettingsHandler(c *gin.Context) {
	// 1. Récupération de l'identité
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 2. Parsage et validation du JSON
	var input conversation_models.UpdateMemberSettingsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou conversation_id manquant.", err))
		return
	}

	// 3. Appel du service métier
	if err := conversation_service.UpdateMemberSettings(c.Request.Context(), userID, input); err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 4. Succès direct, pas besoin d'un DTO de sortie
	c.JSON(http.StatusOK, gin.H{"message": "Paramètres de la conversation mis à jour avec succès"})
}
