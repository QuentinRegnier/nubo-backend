package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// JoinGroupHandler godoc
// @Summary Rejoindre un groupe
// @Description Permet à un utilisateur de rejoindre manuellement un groupe (ex: suite à une invitation en message privé).
// @Description Cette route nécessite une authentification par JWT et une signature HMAC valide.
// @Description
// @Description **Règles de validation & Erreurs :**
// @Description
// @Description 🔴 **400 Bad Request (Erreurs client) :**
// @Description * `Invalid JSON` : Le format JSON est incorrect ou manquant.
// @Description * `Validation failed` : Le payload ne respecte pas les contraintes (ex: conversation_id manquant).
// @Description
// @Description 🟠 **401 Unauthorized (Authentification) :**
// @Description * `Utilisateur non identifié` : Le userID n'a pas pu être extrait du token JWT.
// @Description
// @Description 🟡 **403 Forbidden (Règles Métier) :**
// @Description * L'utilisateur est banni de ce groupe.
// @Description * La conversation n'est pas un groupe (Type 0).
// @Description
// @Description ⚫ **500 Internal Server Error (Serveur) :**
// @Description * Erreur lors de l'insertion dans la file d'attente Redis (Queue).
// @Tags groups
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer <votre_jwt>"
// @Param X-Signature header string true "Signature HMAC de la requête"
// @Param X-Timestamp header string true "Timestamp Unix de la requête"
// @Param data body conversation_models.JoinGroupInput true "Données pour rejoindre"
// @Success 200 {object} map[string]string "Message de succès"
// @Failure 400 {object} nubo_error.PublicErrorResponse "Données invalides"
// @Failure 401 {object} nubo_error.PublicErrorResponse "Session expirée ou utilisateur non identifié"
// @Failure 403 {object} nubo_error.PublicErrorResponse "Accès refusé"
// @Failure 500 {object} nubo_error.PublicErrorResponse "Erreur interne de persistance"
// @Router /group/join [post]
func JoinGroupHandler(c *gin.Context) {
	// 1. Extraction sécurisée de l'identité
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 2. Récupération et validation du Body
	var input conversation_models.JoinGroupInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	// 3. Délégation au Service Métier (Cascade L1->L2->L3 et Write-Behind)
	if err := conversation_service.JoinGroup(c.Request.Context(), callerID, input); err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 4. Succès
	c.JSON(http.StatusOK, gin.H{"message": "Vous avez rejoint le groupe avec succès"})
}
