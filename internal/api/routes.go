package api

import (
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"

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

func SignUpHandler(c *gin.Context) {
	// TODO: lire JSON, enregistrer utilisateur
	c.JSON(http.StatusOK, gin.H{"message": "signup ok"})
}

func LoginHandler(c *gin.Context) {
	// TODO: vérifier identifiants, renvoyer JWT
	c.JSON(http.StatusOK, gin.H{"message": "login ok"})
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
