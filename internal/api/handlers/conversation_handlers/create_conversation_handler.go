package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// CreateConversationHandler godoc
// @Summary      Créer une nouvelle conversation
// @Description  Crée une conversation (MP, Groupe).
// @Description  Pour les MP (Type 0), les règles de confidentialité de la cible sont strictement vérifiées.
// @Description  Pour les Groupes (Type 1), les participants sont ajoutés automatiquement avec gestion des invitations si nécessaire.
// @Description  La création de Communautés (Type 2 et 3) n'est pas autorisée via cette route.
// @Tags         conversations
// @Accept       json
// @Produce      json
// @Param        Authorization header string true  "Bearer <votre_jwt>"
// @Param        X-Signature   header string true  "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true  "Timestamp Unix de la requête"
// @Param        data          body   conversation_models.CreateConversationInput true "Données de la conversation"
// @Success      201  {object} conversation_models.CreateConversationOutput "L'ID de la nouvelle conversation. S'il s'agit d'un groupe (Type 1), inclut les tableaux 'message_ids' et 'conversation_ids' des invitations envoyées."
// @Failure      400  {object} nubo_error.ErrorResponse "Données invalides ou confidentialité refusée"
// @Failure      401  {object} nubo_error.ErrorResponse "Session expirée ou utilisateur non identifié"
// @Router       /conversations [post]
func CreateConversationHandler(c *gin.Context) {
	var input conversation_models.CreateConversationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Format JSON invalide ou champs manquants"})
		return
	}

	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, nubo_error.ErrorResponse{Error: "Utilisateur non identifié"})
		return
	}

	// Appel unique au service métier qui orchestre désormais tout en interne
	output, err := conversation_service.CreateConversation(c.Request.Context(), callerID, input)
	if err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, output)
}
