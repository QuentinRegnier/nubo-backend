package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
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
// @Router       /signup [post]
func SignUpHandler(c *gin.Context) {
	var input domain.SignUpInput
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
	// --- B. MAPPING VERS STRUCTURE INTERNE ---
	var req domain.UserRequest
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
	var mediaID int = -1 // Valeur par défaut "pas d'image"

	if errFile == nil {
		file, err := fileHeader.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Cannot read file"})
			return
		}
		defer file.Close()

		// On récupère l'ID entier de la BDD
		mediaID, err = service.UploadMedia(file, "profile_"+req.Username, "")
		if err != nil {
			fmt.Printf("❌ ERREUR UPLOAD : %v\n", err)
			c.JSON(http.StatusInternalServerError, domain.ErrorResponse{Error: "Internal error (image upload)"})
			return
		}
	}

	req.ProfilePictureID = mediaID

	// --- D. CRÉATION USER & TOKEN ---
	var sessions domain.SessionsRequest
	sessions.ID = -1     // Auto-généré
	sessions.UserID = -1 // Sera défini après création user
	sessions.RefreshToken, err = pkg.GenerateToken(req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{Error: "Internal error (token generation)"})
		return
	}
	sessions.DeviceToken = input.DeviceToken
	sessions.DeviceInfo = input.DeviceInfo
	sessions.IPHistory = []string{c.ClientIP()}
	sessions.CreatedAt = time.Now()
	sessions.ExpiresAt = time.Now().Add(pkg.TIMETOKEN)

	// Persistance en base de données
	// Les arguments 'desactivated', 'banned', etc. sont maintenant DANS 'req'.
	// J'assume que la signature de FuncCreateUser a changé pour accepter (req, token, ...).

	userID, err := service.CreateUser(req, sessions)

	if err == nil {
		//go StartWebsocket()

		c.JSON(http.StatusOK, domain.SignUpResponse{
			UserID:           userID,
			Token:            sessions.RefreshToken,
			ExpiresAt:        sessions.ExpiresAt,
			Message:          "User created successfully",
			ProfilePictureID: req.ProfilePictureID, // On renvoie l'UUID au front pour affichage direct
		})
	} else {
		fmt.Printf("❌ ERREUR CRITIQUE DATABASE (CreateUser): %v\n", err)
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{Error: "database error"})
	}
}

// Login godoc
// @Summary      Connecter un utilisateur
// @Description  Authentifie un utilisateur via email/password et renvoie son profil complet + token.
// @Description
// @Description  **Règles & Erreurs :**
// @Description
// @Description  🔴 **400 Bad Request :**
// @Description  * `The 'data' field containing the JSON is required` : Champ 'data' manquant.
// @Description  * `Invalid JSON format in 'data'` : Le JSON envoyé est mal formé.
// @Description
// @Description  ⚫ **500 Internal Server Error :**
// @Description  * `database error` : Identifiants incorrects ou problème BDD (Note: Idéalement, renvoyer 401 pour mauvais mdp).
// @Tags         users
// @Accept       multipart/form-data
// @Produce      json
// @Param        data formData string true "Données JSON (domain.LoginInput)"
// @Success      200  {object}  domain.LoginResponse
// @Failure      400  {object}  domain.ErrorResponse
// @Failure      500  {object}  domain.ErrorResponse
// @Router       /login [post]
func LoginHandler(c *gin.Context) {
	var input domain.LoginInput
	var user domain.UserRequest
	var sessions domain.SessionsRequest
	var err error

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

	// --- B. MAPPING VERS STRUCTURE INTERNE ---
	user, sessions, err = service.Login(input)

	if err == nil {
		//go StartWebsocket()

		c.JSON(http.StatusOK, domain.LoginResponse{
			UserID:        user.ID,
			Username:      user.Username,
			Email:         user.Email,
			EmailVerified: user.EmailVerified,
			Phone:         user.Phone,
			PhoneVerified: user.PhoneVerified,
			FirstName:     user.FirstName,
			LastName:      user.LastName,
			Birthdate:     user.Birthdate,
			Sex:           user.Sex,
			Bio:           user.Bio,
			Grade:         user.Grade,
			Location:      user.Location,
			School:        user.School,
			Work:          user.Work,
			Badges:        user.Badges,
			Desactivated:  user.Desactivated,
			Banned:        user.Banned,
			BanReason:     user.BanReason,
			BanExpiresAt:  user.BanExpiresAt,
			CreatedAt:     user.CreatedAt,
			UpdatedAt:     user.UpdatedAt,
			Token:         sessions.RefreshToken,
			ExpiresAt:     sessions.ExpiresAt,
			Message:       "Login successful",
		})
	} else {
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{Error: "database error"})
	}
}
