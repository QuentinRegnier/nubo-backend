package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/QuentinRegnier/nubo-backend/internal/data"
	"github.com/QuentinRegnier/nubo-backend/internal/db"
	"github.com/QuentinRegnier/nubo-backend/internal/media"
	"github.com/QuentinRegnier/nubo-backend/internal/tools"
	"github.com/QuentinRegnier/nubo-backend/internal/websocket"
	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) {
	// Routes REST ...
	r.POST("/signup", SignUpHandler)
	r.POST("/login", LoginHandler)
	r.GET("/posts", GetPostsHandler)
	r.POST("/posts", CreatePostHandler)

	// WebSocket
	r.GET("/token", func(c *gin.Context) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": "user123",
			"exp": time.Now().Add(time.Hour * 24).Unix(), // expire dans 24h
		})
		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			panic("JWT_SECRET manquant dans .env")
		}
		tokenString, _ := token.SignedString([]byte(secret))
		c.JSON(200, gin.H{"token": tokenString})
	})
	r.GET("/ws", JWTMiddleware(), websocket.WSHandler)
}

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
// @Param        data            formData string true  "Données JSON (tools.SignUpInput)"
// @Success      200  {object}  tools.SignUpResponse
// @Failure      400  {object}  tools.ErrorResponse "Données invalides (Voir liste ci-dessus)"
// @Failure      409  {object}  tools.ErrorResponse "Conflit (Pseudo pris)"
// @Failure      500  {object}  tools.ErrorResponse "Erreur Serveur"
// @Router       /signup [post]
func SignUpHandler(c *gin.Context) {
	var input tools.SignUpInput
	// --- A. RÉCUPÉRATION DES DONNÉES MIXTES (Multipart) ---
	jsonData := c.PostForm("data")
	if jsonData == "" {
		c.JSON(http.StatusBadRequest, tools.ErrorResponse{Error: "The 'data' field containing the JSON is required"})
		return
	}
	if err := json.Unmarshal([]byte(jsonData), &input); err != nil {
		c.JSON(http.StatusBadRequest, tools.ErrorResponse{Error: "Invalid JSON format in 'data': " + err.Error()})
		return
	}
	// --- B. MAPPING VERS STRUCTURE INTERNE ---
	var req tools.UserRequest
	req.ID = -1
	req.Username = input.Username
	if data.IsUnique(db.Users, "username", req.Username) == 0 {
		c.JSON(http.StatusConflict, tools.ErrorResponse{Error: "This username is already taken"})
		return
	}
	req.Email = input.Email
	req.EmailVerified = false // Par défaut
	req.Phone = input.Phone
	req.PhoneVerified = false // Par défaut
	req.PasswordHash = input.PasswordHash
	req.FirstName = input.FirstName
	req.LastName = input.LastName
	parsedBirthdate, err := time.Parse("02012006", input.Birthdate)
	if err != nil {
		c.JSON(http.StatusBadRequest, tools.ErrorResponse{Error: "Invalid date format. Expected format: ddmmaaaa"})
		return
	}
	req.Birthdate = parsedBirthdate
	if input.Gender != nil {
		g := *input.Gender
		if g < 0 || g > 2 {
			c.JSON(http.StatusBadRequest, tools.ErrorResponse{Error: "Gender must be 0, 1, 2, or null"})
			return
		}
		req.Sex = g
	} else {
		// Gérer le cas null si nécessaire, par défaut int vaut 0.
		// Si 0 est une valeur valide (ex: Homme), il faut définir une logique pour "Non spécifié".
	}
	req.Bio = tools.CleanStr(input.Bio) // Nettoyage immédiat
	req.Grade = 0                       // Par défaut
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
			c.JSON(http.StatusBadRequest, tools.ErrorResponse{Error: "Cannot read file"})
			return
		}
		defer file.Close()

		// On récupère l'ID entier de la BDD
		mediaID, err = media.UploadMedia(file, "profile_"+req.Username, "")
		if err != nil {
			fmt.Printf("❌ ERREUR UPLOAD : %v\n", err)
			c.JSON(http.StatusInternalServerError, tools.ErrorResponse{Error: "Internal error (image upload)"})
			return
		}
	}

	req.ProfilePictureID = mediaID

	// --- D. CRÉATION USER & TOKEN ---
	var sessions tools.SessionsRequest
	sessions.ID = -1     // Auto-généré
	sessions.UserID = -1 // Sera défini après création user
	sessions.RefreshToken, err = tools.GenerateToken(req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, tools.ErrorResponse{Error: "Internal error (token generation)"})
		return
	}
	sessions.DeviceToken = input.DeviceToken
	sessions.DeviceInfo = input.DeviceInfo
	sessions.IPHistory = []string{c.ClientIP()}
	sessions.CreatedAt = time.Now()
	sessions.ExpiresAt = time.Now().Add(tools.TIMETOKEN)

	// Persistance en base de données
	// Les arguments 'desactivated', 'banned', etc. sont maintenant DANS 'req'.
	// J'assume que la signature de FuncCreateUser a changé pour accepter (req, token, ...).

	userID, err := data.CreateUser(req, sessions)

	if err == nil {
		//go StartWebsocket()

		c.JSON(http.StatusOK, tools.SignUpResponse{
			UserID:           userID,
			Token:            sessions.RefreshToken,
			ExpiresAt:        sessions.ExpiresAt,
			Message:          "User created successfully",
			ProfilePictureID: req.ProfilePictureID, // On renvoie l'UUID au front pour affichage direct
		})
	} else {
		fmt.Printf("❌ ERREUR CRITIQUE DATABASE (CreateUser): %v\n", err)
		c.JSON(http.StatusInternalServerError, tools.ErrorResponse{Error: "database error"})
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
// @Param        data formData string true "Données JSON (tools.LoginInput)"
// @Success      200  {object}  tools.LoginResponse
// @Failure      400  {object}  tools.ErrorResponse
// @Failure      500  {object}  tools.ErrorResponse
// @Router       /login [post]
func LoginHandler(c *gin.Context) {
	var input tools.LoginInput
	var user tools.UserRequest
	var sessions tools.SessionsRequest
	var err error

	// --- A. RÉCUPÉRATION DES DONNÉES MIXTES (Multipart) ---
	jsonData := c.PostForm("data")
	if jsonData == "" {
		c.JSON(http.StatusBadRequest, tools.ErrorResponse{Error: "The 'data' field containing the JSON is required"})
		return
	}
	if err := json.Unmarshal([]byte(jsonData), &input); err != nil {
		c.JSON(http.StatusBadRequest, tools.ErrorResponse{Error: "Invalid JSON format in 'data': " + err.Error()})
		return
	}

	// --- B. MAPPING VERS STRUCTURE INTERNE ---
	user, sessions, err = data.Login(input)

	if err == nil {
		//go StartWebsocket()

		c.JSON(http.StatusOK, tools.LoginResponse{
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
		c.JSON(http.StatusInternalServerError, tools.ErrorResponse{Error: "database error"})
	}
}

func GetPostsHandler(c *gin.Context) {
	// TODO: récupérer les posts depuis la base
	c.JSON(http.StatusOK, gin.H{"posts": []string{"post 1", "post 2"}})
}

func CreatePostHandler(c *gin.Context) {
	// TODO: ajouter post à la base
	c.JSON(http.StatusCreated, gin.H{"message": "post created"})
}

func LoadMorePostsHandler(c *gin.Context) {
	// TODO: charger plus de posts depuis la base
	c.JSON(http.StatusOK, gin.H{"posts": []string{"post 3", "post 4"}})
}

func LikeHandler(c *gin.Context) {
	// TODO: ajouter un like à un post
	c.JSON(http.StatusOK, gin.H{"message": "post liked"})
}

func CommentHandler(c *gin.Context) {
	// TODO: ajouter un commentaire à un post
	c.JSON(http.StatusOK, gin.H{"message": "post commented"})
}

func SignalHandler(c *gin.Context) {
	// TODO: gérer les signaux
	c.JSON(http.StatusOK, gin.H{"message": "signal received"})
}

func LoadCommentsHandler(c *gin.Context) {
	// TODO: charger les commentaires d'un post depuis la base
	c.JSON(http.StatusOK, gin.H{"comments": []string{"comment 1", "comment 2"}})
}

func UnlikeHandler(c *gin.Context) {
	// TODO: retirer un like à un post
	c.JSON(http.StatusOK, gin.H{"message": "post unliked"})
}

func UncommentHandler(c *gin.Context) {
	// TODO: retirer un commentaire à un post
	c.JSON(http.StatusOK, gin.H{"message": "post uncommented"})
}

func ModifyPostHandler(c *gin.Context) {
	// TODO: modifier un post dans la base
	c.JSON(http.StatusOK, gin.H{"message": "post modified"})
}

func BanHandler(c *gin.Context) {
	// TODO: gérer les bans
	c.JSON(http.StatusOK, gin.H{"message": "user banned"})
}

func ProfilePictureHandler(c *gin.Context) {
	// TODO: gérer la mise à jour de la photo de profil
	c.JSON(http.StatusOK, gin.H{"message": "profile picture updated"})
}

func DescriptionHandler(c *gin.Context) {
	// TODO: gérer la mise à jour de la description
	c.JSON(http.StatusOK, gin.H{"message": "description updated"})
}

func LocalisationHandler(c *gin.Context) {
	// TODO: gérer la mise à jour de la localisation
	c.JSON(http.StatusOK, gin.H{"message": "localisation updated"})
}

func StudyHandler(c *gin.Context) {
	// TODO: gérer la mise à jour des informations d'études
	c.JSON(http.StatusOK, gin.H{"message": "study information updated"})
}

func WorkHandler(c *gin.Context) {
	// TODO: gérer la mise à jour des informations professionnelles
	c.JSON(http.StatusOK, gin.H{"message": "work information updated"})
}

func ConfidentialityHandler(c *gin.Context) {
	// TODO: gérer la mise à jour des paramètres de confidentialité
	c.JSON(http.StatusOK, gin.H{"message": "confidentiality settings updated"})
}

func UsernameHandler(c *gin.Context) {
	// TODO: gérer la mise à jour du nom d'utilisateur
	c.JSON(http.StatusOK, gin.H{"message": "username updated"})
}

func NameHandler(c *gin.Context) {
	// TODO: gérer la mise à jour du nom
	c.JSON(http.StatusOK, gin.H{"message": "name updated"})
}

func LastNameHandler(c *gin.Context) {
	// TODO: gérer la mise à jour du nom de famille
	c.JSON(http.StatusOK, gin.H{"message": "last name updated"})
}

func LanguageHandler(c *gin.Context) {
	// TODO: gérer la mise à jour de la langue
	c.JSON(http.StatusOK, gin.H{"message": "language updated"})
}

func ConversationHandler(c *gin.Context) {
	// TODO: gérer la création d'une nouvelle conversation
	c.JSON(http.StatusOK, gin.H{"message": "new conversation created"})
}

func GroupHandler(c *gin.Context) {
	// TODO: gérer la création d'un nouveau groupe
	c.JSON(http.StatusOK, gin.H{"message": "new group created"})
}

func UserGroupHandler(c *gin.Context) {
	// TODO: gérer la récupération des groupes d'un utilisateur
	c.JSON(http.StatusOK, gin.H{"groups": []string{"group 1", "group 2"}})
}

func AdminGroupHandler(c *gin.Context) {
	// TODO: gérer les administrateurs d'un groupe
	c.JSON(http.StatusOK, gin.H{"message": "group admins retrieved"})
}

func MessageHandler(c *gin.Context) {
	// TODO: gérer l'envoi d'un message
	c.JSON(http.StatusOK, gin.H{"message": "message sent"})
}

func LoadImageHandler(c *gin.Context) {
	// TODO: gérer le chargement d'une image
	c.JSON(http.StatusOK, gin.H{"message": "image loaded"})
}

func FriendShipHandler(c *gin.Context) {
	// TODO: gérer les relations d'amitié
	c.JSON(http.StatusOK, gin.H{"message": "friendship managed"})
}

func LoadNewMessagesHandler(c *gin.Context) {
	// TODO: gérer le chargement des nouveaux messages
	c.JSON(http.StatusOK, gin.H{"messages": []string{"new message 1", "new message 2"}})
}
