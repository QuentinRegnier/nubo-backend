package auth_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/auth_service"
	"github.com/gin-gonic/gin"
)

// UpdateProfileHandler godoc
// @Summary      Mettre à jour le profil public
// @Description  Met à jour les informations de profil (prénom, nom, bio, etc.) et remplace la photo de profil.
// @Description  Cette route nécessite une authentification par JWT et une signature HMAC valide.
// @Description
// @Description  **Règles de validation & Erreurs :**
// @Description
// @Description  🔴 **400 Bad Request (Erreurs client) :**
// @Description  * `Invalid JSON in form data: ...` : Le format JSON dans le champ 'data' est incorrect.
// @Description  * `Invalid JSON: ...` : Le format du JSON classique est incorrect.
// @Description
// @Description  🟠 **401 Unauthorized (Authentification) :**
// @Description  * `Non autorisé` : Le userID n'a pas pu être extrait du token JWT.
// @Description
// @Description  🟡 **409 Conflict (Règles métier) :**
// @Description  * `ce nom d'utilisateur est déjà pris` : Conflit détecté (Cuckoo Filter / BDD).
// @Description  * `cet email est déjà utilisé` : Conflit d'email.
// @Description  * `ce numéro de téléphone est déjà utilisé` : Conflit de téléphone.
// @Description
// @Description  ⚫ **500 Internal Server Error (Serveur) :**
// @Description  * `Impossible de lire le fichier` : Erreur de lecture de l'avatar.
// @Description  * `erreur upload avatar` : Échec lors de la communication avec le stockage S3 (MinIO).
// @Tags         Profil
// @Accept       multipart/form-data
// @Accept       application/json
// @Produce      json
// @Param        Authorization   header   string true  "Bearer <votre_jwt>"
// @Param        X-Signature     header   string true  "Signature HMAC de la requête"
// @Param        X-Timestamp     header   string true  "Timestamp Unix de la requête"
// @Param        profile_picture formData file   false "Nouvelle image de profil (optionnelle)"
// @Param        data            formData string false "Données JSON (auth_models.UpdateProfileInput) si multipart"
// @Param        payload         body     auth_models.UpdateProfileInput false "Données JSON classiques si pas d'image"
// @Success      200 {object} map[string]string "Profil mis à jour avec succès"
// @Failure      400 {object} nubo_error.PublicErrorResponse "JSON invalide"
// @Failure      401 {object} nubo_error.PublicErrorResponse "Non autorisé"
// @Failure      409 {object} nubo_error.PublicErrorResponse "Conflit d'identifiant (Username, Email, Phone)"
// @Failure      500 {object} nubo_error.PublicErrorResponse "Erreur interne (Upload S3, etc.)"
// @Router       /profile [patch]
func UpdateProfileHandler(c *gin.Context) {
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input auth_models.UpdateProfileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide.", err))
		return
	}

	if err := auth_service.UpdateProfile(c.Request.Context(), userID, input); err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Profil mis à jour avec succès"})
}
