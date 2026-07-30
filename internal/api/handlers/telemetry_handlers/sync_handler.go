package telemetry_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/telemetry_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/telemetry_service"
	"github.com/gin-gonic/gin"
)

// SyncHandler godoc
// @Summary      Synchronisation Edge-to-Cloud (Télémétrie & Profil)
// @Description  Reçoit les Dwell Times, clics et le vecteur de préférences local. Gère la résolution de conflits (Multi-appareils).
// @Tags         telemetry
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        data          body   telemetry_models.SyncPayload true "Données de télémétrie et vecteur"
// @Success      200  {object}  telemetry_models.SyncOutput "Le serveur a des données plus récentes, mise à jour du client requise"
// @Success      202  {object}  telemetry_models.SyncOutput "Données acceptées et traitées"
// @Failure      400  {object}  domain.ErrorResponse "Format JSON invalide"
// @Failure      401  {object}  domain.ErrorResponse "Utilisateur non identifié"
// @Router       /telemetry/sync [patch]
func SyncHandler(c *gin.Context) {
	// 1. SÉCURITÉ
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, nubo_error.ErrorResponse{Error: "Utilisateur non identifié"})
		return
	}

	// 2. PARSING DU PAYLOAD
	var payload telemetry_models.SyncPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Format JSON invalide"})
		return
	}

	// 3. APPEL DU SERVICE
	input := telemetry_models.SyncInput{
		UserID:  userID,
		Payload: payload,
	}

	output, err := telemetry_service.ProcessSync(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, nubo_error.ErrorResponse{Error: "Erreur interne lors de la synchronisation"})
		return
	}

	// 4. RÉPONSE DYNAMIQUE (200 OK vs 202 Accepted)
	if output.NeedUpdate {
		c.JSON(http.StatusOK, output) // Le client doit lire le body et se mettre à jour
	} else {
		c.JSON(http.StatusAccepted, output) // On a accepté et processé sa donnée en async
	}
}
