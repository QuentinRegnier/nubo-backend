package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// PromoteMemberHandler godoc
// @Summary      Promouvoir un membre administrateur
// @Description  Promeut un membre standard d'un groupe au rang d'administrateur.
// @Description  Seul le propriétaire du groupe (rôle = 2) est autorisé à effectuer cette action.
// @Description  La mise à jour de la mémoire RAM L1 est instantanée et asynchrone pour les bases de données.
// @Tags         conversations
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        data          body   conversation_models.PromoteMemberInput true "ID du groupe et de l'utilisateur à promouvoir"
// @Success      200  {object} map[string]string "message: Membre promu avec succès"
// @Failure      400  {object} nubo_error.ErrorResponse "Format JSON invalide ou champs manquants"
// @Failure      401  {object} nubo_error.ErrorResponse "Utilisateur non identifié"
// @Failure      403  {object} nubo_error.ErrorResponse "Seul le propriétaire peut promouvoir un membre"
// @Router       /conversations/promote [put]
func PromoteMemberHandler(c *gin.Context) {
	// 1. Authentification
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, nubo_error.ErrorResponse{Error: "Utilisateur non identifié"})
		return
	}

	// 2. Validation du JSON
	var input conversation_models.PromoteMemberInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Format JSON invalide ou paramètres manquants"})
		return
	}

	// 3. Appel du Service
	if err := conversation_service.PromoteMember(c.Request.Context(), callerID, input); err != nil {
		c.JSON(http.StatusForbidden, nubo_error.ErrorResponse{Error: err.Error()})
		return
	}

	// 4. Succès
	c.JSON(http.StatusOK, gin.H{"message": "Membre promu avec succès"})
}
