package auth_handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/auth_service"
	"github.com/gin-gonic/gin"
)

// SignUp godoc
// @Summary      Créer un compte utilisateur
// @Description  Inscription complète avec upload d'avatar et données JSON.
// @Description
// @Description  **Règles de validation & Erreurs :**
// @Description
// @Description  🔴 **400 Bad Request (Erreurs client) :**
// @Description  * `The 'data' field containing the JSON is required` : Tu as oublié d'envoyer le champ texte 'data'.
// @Description  * `Invalid JSON format in 'data': ...` : Ton JSON est mal écrit (virgule manquante, accolade, etc).
// @Description  * `Invalid date format. Expected format: ddmmaaaa` : La date de naissance n'est pas bonne.
// @Description  * `Gender must be 0, 1, 2, or null` : Tu as envoyé un entier invalide pour le sexe.
// @Description  * `Impossible to read image file` : Le fichier image est corrompu ou illisible.
// @Description  * `You must be at least 13 years old` : Restrictions d'âge.
// @Description  * `Invalid birthdate` : Date absurde (ex: plus de 120 ans).
// @Description
// @Description  🟠 **409 Conflict (Doublons) :**
// @Description  * `This username is already taken` : Le pseudo est déjà en base.
// @Description  * `This email is already taken` : L'email est déjà en base.
// @Description  * `This phone number is already taken` : Le téléphone est déjà en base.
// @Description
// @Description  ⚫ **500 Internal Server Error (Problèmes serveur) :**
// @Description  * `Internal nubo_error (image upload)` : MinIO est down ou mal configuré.
// @Description  * `Internal nubo_error (token generation)` : Problème avec la signature JWT.
// @Description  * `database nubo_error` : Postgres ou Mongo ne répondent pas.
// @Tags         users
// @Accept       multipart/form-data
// @Produce      json
// @Param        profile_picture formData file   false "Photo de profil (Image)"
// @Param        data            formData string true  "Données JSON (domain.SignUpInput)"
// @Success      200  {object}  auth_models.SignUpResponse
// @Failure      400  {object}  domain.ErrorResponse "Données invalides (Voir liste ci-dessus)"
// @Failure      409  {object}  domain.ErrorResponse "Conflit (Pseudo pris)"
// @Failure      500  {object}  domain.ErrorResponse "Erreur Serveur"
// @Router       /signup [post]
func SignUpHandler(c *gin.Context) {
	var input auth_models.SignUpInput

	// --- 1. RÉCUPÉRATION DES DONNÉES MIXTES (Multipart) ---
	jsonData := c.PostForm("data")
	if jsonData == "" {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "The 'data' field containing the JSON is required"})
		return
	}

	if err := json.Unmarshal([]byte(jsonData), &input); err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Invalid JSON format in 'data': " + err.Error()})
		return
	}

	// --- 2. 🛡️ BOUCLIER STATIQUE : Validation O(1) ---
	if err := pkg.ValidateStruct(&input); err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Validation failed: " + err.Error()})
		return
	}

	// --- 3. RÉCUPÉRATION MÉDIA ---
	fileHeader, errFile := c.FormFile("profile_picture")

	// --- 4. APPEL AU SERVICE MÉTIER ---
	// L'IP est transmise au service pour la traçabilité de la session
	response, err := auth_service.CreateUser(input, c.ClientIP(), fileHeader, errFile)

	if err != nil {
		// Routage strict des erreurs métier vers les statuts HTTP appropriés
		switch err {
		case nubo_error.ErrUsernameTaken, nubo_error.ErrEmailTaken, nubo_error.ErrPhoneTaken:
			c.JSON(http.StatusConflict, nubo_error.ErrorResponse{Error: err.Error()})
		case nubo_error.ErrInvalidDate, nubo_error.ErrAgeUnder13, nubo_error.ErrAgeOver120, nubo_error.ErrInvalidGender:
			c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: err.Error()})
		default:
			fmt.Printf("❌ ERREUR CRITIQUE SERVEUR (CreateUser): %v\n", err)
			c.JSON(http.StatusInternalServerError, nubo_error.ErrorResponse{Error: "database nubo_error"})
		}
		return
	}

	// --- 5. SUCCÈS HTTP 200 ---
	// go StartWebsocket() // A décommenter selon tes besoins
	c.JSON(http.StatusOK, response)
}
