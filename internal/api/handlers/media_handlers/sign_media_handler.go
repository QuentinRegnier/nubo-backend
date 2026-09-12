package media_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/gin-gonic/gin"
)

// SignMediaHandler godoc
// @Summary      Échanger un MediaID contre une URL signée (Claim Check)
// @Description  Fournit une URL HMAC sécurisée pour afficher un média lourd. Vérifie les droits d'accès au contexte (Post ou Conversation) en temps réel (Zero-Trust).
// @Tags         media
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        data          body   media_models.SignMediaInput true "Les informations du média (ID) et son contexte d'apparition (PostID ou ConversationID)"
// @Success      200  {object} media_models.SignMediaOutput "L'URL signée HMAC prête à l'emploi"
// @Failure      400  {object} nubo_error.PublicErrorResponse "Données invalides, contexte ambigu ou manquant"
// @Failure      401  {object} nubo_error.PublicErrorResponse "Utilisateur non identifié ou token expiré"
// @Failure      403  {object} nubo_error.PublicErrorResponse "Accès refusé au contexte (Banni du post ou de la conversation)"
// @Failure      404  {object} nubo_error.PublicErrorResponse "Média introuvable"
// @Router       /media/sign [post]
func SignMediaHandler(c *gin.Context) {
	// 1. Extraction du ReaderID (l'utilisateur qui demande à voir l'image)
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 2. Parsing du payload JSON
	var input media_models.SignMediaInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou paramètre media_id manquant.", err))
		return
	}

	// 3. Bouclier contextuel (Anti-ambiguïté)
	// Un média s'affiche soit dans un post, soit dans un message, mais pas les deux en même temps.
	if input.PostID > 0 && input.ConversationID > 0 {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("AMBIGUOUS_CONTEXT", "Veuillez fournir soit un post_id soit un conversation_id, mais pas les deux.", nil))
		return
	}

	// 4. Appel du service de signature (qui effectue le contrôle Zero-Trust L1->L2->L3)
	output, err := media_service.GetSignedURLForClient(c.Request.Context(), callerID, input.MediaID, input.PostID, input.ConversationID)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 5. Renvoi du "ticket" valide à l'application mobile
	c.JSON(http.StatusOK, output)
}
