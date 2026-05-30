package auth_handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/auth_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
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
// @Description
// @Description  🟠 **409 Conflict (Doublons) :**
// @Description  * `This username is already taken` : Le pseudo est déjà en base.
// @Description
// @Description  ⚫ **500 Internal Server Error (Problèmes serveur) :**
// @Description  * `Internal error (image upload)` : MinIO est down ou mal configuré.
// @Description  * `Internal error (token generation)` : Problème avec la signature JWT.
// @Description  * `database error` : Postgres ou Mongo ne répondent pas.
// @Tags         users
// @Accept       multipart/form-data
// @Produce      json
// @Param        profile_picture formData file   false "Photo de profil (Image)"
// @Param        data            formData string true  "Données JSON (domain.SignUpInput)"
// @Success      200  {object}  domain.SignUpResponse
// @Failure      400  {object}  domain.ErrorResponse "Données invalides (Voir liste ci-dessus)"
// @Failure      409  {object}  domain.ErrorResponse "Conflit (Pseudo pris)"
// @Failure      500  {object}  domain.ErrorResponse "Erreur Serveur"
// @Router       /signup [post_service]
func SignUpHandler(c *gin.Context) {
	var input auth_models.SignUpInput
	// --- A. RÉCUPÉRATION DES DONNÉES MIXTES (Multipart) ---
	jsonData := c.PostForm("data")
	if jsonData == "" {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "The 'data' field containing the JSON is required"})
		return
	}
	if err := json.Unmarshal([]byte(jsonData), &input); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Invalid JSON format in 'data': " + err.Error()})
		return
	}

	// 🛡️ BOUCLIER STATIQUE : Validation O(1) des formats et longueurs
	if err := pkg.ValidateStruct(&input); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Validation failed: " + err.Error()})
		return
	}

	// --- B. MAPPING VERS STRUCTURE INTERNE ---
	var req models.UserRequest
	req.ID = -1
	req.Username = input.Username
	if service.IsUnique(mongo.Users, "username", req.Username) == 0 {
		c.JSON(http.StatusConflict, domain.ErrorResponse{Error: "This username is already taken"})
		return
	}
	req.Email = input.Email
	if service.IsUnique(mongo.Users, "email", req.Email) == 0 {
		c.JSON(http.StatusConflict, domain.ErrorResponse{Error: "This email is already taken"})
		return
	}
	req.EmailVerified = false // Par défaut
	req.Phone = input.Phone
	if service.IsUnique(mongo.Users, "phone", req.Phone) == 0 {
		c.JSON(http.StatusConflict, domain.ErrorResponse{Error: "This phone number is already taken"})
		return
	}
	req.PhoneVerified = false // Par défaut
	req.PasswordHash = input.PasswordHash
	req.FirstName = input.FirstName
	req.LastName = input.LastName
	parsedBirthdate, err := time.Parse("02012006", input.Birthdate)
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Invalid date format. Expected format: ddmmaaaa"})
		return
	}

	// 🛡️ BOUCLIER STATIQUE : Vérification de l'âge (O(1))
	age := time.Since(parsedBirthdate).Hours() / 24 / 365
	if age < 13 {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "You must be at least 13 years old"})
		return
	}
	if age > 120 {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Invalid birthdate"})
		return
	}

	req.Birthdate = parsedBirthdate
	if input.Gender != nil {
		g := *input.Gender
		if g < 0 || g > 2 {
			c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Gender must be 0, 1, 2, or null"})
			return
		}
		req.Sex = g
	} else {
		// Gérer le cas null si nécessaire, par défaut int vaut 0.
		// Si 0 est une valeur valide (ex: Homme), il faut définir une logique pour "Non spécifié".
	}
	req.Bio = pkg.CleanStr(input.Bio) // Nettoyage immédiat
	req.Grade = 0                     // Par défaut
	req.Location = input.Location
	req.School = input.School
	req.Work = input.Work
	req.Badges = []string{}
	req.Desactivated = false // Par défaut
	req.Banned = false       // Par défaut
	req.BanReason = ""
	req.BanExpiresAt = time.Time{}
	req.CreatedAt = time.Time{}
	req.UpdatedAt = time.Time{}

	// --- C. LOGIQUE UPLOAD ---
	fileHeader, errFile := c.FormFile("profile_picture")

	// --- D. CRÉATION USER & TOKEN ---
	var sessions models.SessionsRequest
	sessions.ID = -1     // Auto-généré
	sessions.UserID = -1 // Sera défini après création user
	sessions.MasterToken = ""
	sessions.DeviceToken = input.DeviceToken
	sessions.DeviceInfo = input.DeviceInfo
	sessions.IPHistory = []string{c.ClientIP()}
	sessions.CurrentSecret = ""
	sessions.LastSecret = sessions.DeviceToken
	sessions.LastJWT = ""
	sessions.ToleranceTime = time.Now().Add(time.Duration(variables.ToleranceTimeSeconds) * time.Second)
	sessions.CreatedAt = time.Time{}
	sessions.ExpiresAt = time.Now().Add(time.Duration(variables.MasterTokenExpirationSeconds) * time.Second)

	userID, JWT, err := auth_service.CreateUser(&req, &sessions, fileHeader, errFile)

	if err == nil {
		//go StartWebsocket()

		c.JSON(http.StatusOK, auth_models.SignUpResponse{
			UserID:           userID,
			MasterToken:      sessions.MasterToken,
			JWT:              JWT,
			ExpiresAt:        sessions.ExpiresAt,
			Message:          "User created successfully",
			ProfilePictureID: req.ProfilePictureID, // On renvoie l'UUID au front pour affichage direct
		})
	} else {
		fmt.Printf("❌ ERREUR CRITIQUE DATABASE (CreateUser): %v\n", err)
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{Error: "database error"})
	}
}
