package api

import (
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/QuentinRegnier/nubo-backend/internal/api/handlers"
	"github.com/QuentinRegnier/nubo-backend/internal/api/middleware"
	"github.com/QuentinRegnier/nubo-backend/internal/api/websocket"
	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) {
	// =========================================================================
	// 0. Middleware GLOBAL (S'applique à TOUTES les routes)
	// =========================================================================

	// Active la gestion automatique des OPTIONS et du CORS
	r.Use(middleware.CORSMiddleware())

	// Récupération automatique des panics (évite que le serveur crash totalement)
	r.Use(gin.Recovery())

	// =========================================================================
	// 1. ROUTES PUBLIQUES (Aucune sécu ou sécu spécifique interne)
	// =========================================================================

	// Authentification (Sécu interne spécifique)
	r.POST("/signup", handlers.SignUpHandler)
	r.POST("/login", handlers.LoginHandler)

	// Renouvellement de Tokens (Ratchet / Master)
	// Ces routes gèrent leur propre sécurité (HMAC spécial, checks BDD...)
	r.POST("/renew-jwt", handlers.RenewJWT)
	r.POST("/refresh-master", handlers.RefreshMaster)

	// WebSocket
	r.GET("/token", func(c *gin.Context) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": 1234,
			"dev": "device-token-sample",
			"exp": time.Now().Add(time.Hour * 24).Unix(), // expire dans 24h
			"iat": time.Now().Unix(),
		})
		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			panic("JWT_SECRET manquant dans .env")
		}
		tokenString, _ := token.SignedString([]byte(secret))
		c.JSON(200, gin.H{"token": tokenString})
	})

	// WebSocket Connection (Sécu via Query param ou Header standard, géré par le handler WS)
	// Note: Si tu veux JWT pour le WS, tu peux utiliser le middleware ici ou dans le handler.
	// Pour l'instant, je le laisse avec le middleware JWT comme dans ton exemple.
	r.GET("/ws", middleware.JWTMiddleware(), websocket.WSHandler)

	// =========================================================================
	// 2. ROUTES SÉCURISÉES (JWT + HMAC + RATCHET)
	// Toutes les routes ci-dessous nécessitent :
	// - Un JWT valide (Identity)
	// - Une signature HMAC valide (Integrity & Anti-Replay)
	// =========================================================================

	// On crée un groupe "plat" qui applique les deux middlewares d'un coup
	secured := r.Group("/")
	secured.Use(middleware.JWTMiddleware())  // 1. Qui est-ce ? (Populate context with UserID & DeviceToken)
	secured.Use(middleware.HMACMiddleware()) // 2. Est-ce authentique ? (Check Signature with Redis Secret)

	// --- Posts ---
	secured.GET("/posts", GetPostsHandler)
	secured.POST("/posts", CreatePostHandler)
	secured.GET("/posts/more", LoadMorePostsHandler) // J'invente l'URL pour "LoadMore"
	secured.PUT("/posts", ModifyPostHandler)
	//secured.DELETE("/posts/:id", handlers.DeletePost) // Exemple si tu l'as

	// --- Actions Sociales ---
	secured.POST("/like", LikeHandler)
	secured.DELETE("/like", UnlikeHandler)
	secured.POST("/comment", CommentHandler)
	secured.DELETE("/comment", UncommentHandler)
	secured.GET("/comments", LoadCommentsHandler)
	secured.POST("/signal", SignalHandler)

	// --- Profil Utilisateur ---
	secured.PUT("/profile/picture", ProfilePictureHandler)
	secured.PUT("/profile/description", DescriptionHandler)
	secured.PUT("/profile/localisation", LocalisationHandler)
	secured.PUT("/profile/study", StudyHandler)
	secured.PUT("/profile/work", WorkHandler)
	secured.PUT("/profile/confidentiality", ConfidentialityHandler)
	secured.PUT("/profile/username", UsernameHandler)
	secured.PUT("/profile/name", NameHandler)
	secured.PUT("/profile/lastname", LastNameHandler)
	secured.PUT("/profile/language", LanguageHandler)

	// --- Administration / Modération ---
	secured.POST("/ban", BanHandler)

	// --- Messagerie / Groupes ---
	secured.POST("/conversation", ConversationHandler)
	secured.POST("/group", GroupHandler)
	secured.GET("/user/groups", UserGroupHandler)
	secured.GET("/group/admins", AdminGroupHandler)
	secured.POST("/message", MessageHandler)
	secured.GET("/messages/new", LoadNewMessagesHandler)
	secured.POST("/image/upload", LoadImageHandler)
	secured.POST("/friendship", FriendShipHandler)
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
