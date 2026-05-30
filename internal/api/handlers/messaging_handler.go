package handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/gin-gonic/gin"
)

// InboxHandler godoc
// @Summary      Chargement rapide de la boîte de réception (Inbox)
// @Description  Récupère instantanément la liste des conversations triées par activité chronologique récente (SPEED Cache).
// @Description  Assemble en un seul MGET les métadonnées de la conversation et l'état de l'utilisateur (pastille de notification).
// @Tags         messaging
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Success      200  {array}   service.InboxItemView "Tableau des conversations assemblées pour l'Inbox"
// @Failure      401  {object}  domain.ErrorResponse "Non autorisé (Token invalide ou manquant)"
// @Failure      500  {object}  domain.ErrorResponse "Erreur interne lors de l'assemblage de l'Inbox"
// @Router       /inbox [get]
func InboxHandler(c *gin.Context) {
	// 1. Identification
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{Error: "Utilisateur non identifié"})
		return
	}

	// 2. Appel du pipeline d'hydratation
	inbox, err := cache_service.GetInboxView(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{Error: "Erreur lors du chargement de l'inbox"})
		return
	}

	// Garantie JSON tableau vide [] plutôt que 'null'
	if inbox == nil {
		inbox = []cache_service.InboxItemView{}
	}

	c.JSON(http.StatusOK, inbox)
}
