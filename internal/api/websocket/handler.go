package websocket

import (
	"context"
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // À affiner selon ta config CORS plus tard
	},
}

// ServeWS gère la requête HTTP de la PWA et la transforme en WebSocket
func ServeWS(c *gin.Context) {
	// L'identité est extraite du JWT validé par le Middleware !
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil || userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"nubo_error": "Non autorisé"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return // L'erreur est gérée par upgrader et renvoyée au client
	}

	client := &Client{
		Hub:    GlobalHub,
		Conn:   conn,
		UserID: userID,
		Send:   make(chan []byte, 256), // Buffer de 256 messages max avant d'expulser le client lent
	}

	client.Hub.Register <- client

	// Marque la présence initiale en RAM
	_ = cache_service.MarkUserOnline(context.Background(), userID)

	// Démarrage des pompes asynchrones !
	go client.WritePump()
	go client.ReadPump()
}
