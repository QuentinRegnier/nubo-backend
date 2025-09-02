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
