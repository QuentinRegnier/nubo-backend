package websocket

import (
	"context"
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
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
	// L'identité est extraite du JWT validé par le Middleware
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil || userID == 0 {
		nubo_error.RespondWithError(c, nubo_error.NewUnauthorized("UNAUTHORIZED", "Non autorisé.", err))
		return
	}

	// NOUVEAU : Récupération du Device ID injecté par le JWTMiddleware
	deviceIDRaw, exists := c.Get("firebaseInstallationID")
	if !exists {
		nubo_error.RespondWithError(c, nubo_error.NewUnauthorized("MISSING_DEVICE_ID", "Device ID introuvable dans le contexte.", nil))
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return // L'erreur est gérée par upgrader et renvoyée au client via HTTP
	}

	client := &Client{
		Hub:      GlobalHub,
		Conn:     conn,
		UserID:   userID,
		DeviceID: deviceIDRaw.(string),
		Send:     make(chan []byte, 256),
	}

	client.Hub.Register <- client
	_ = cache_service.MarkUserOnline(context.Background(), userID)

	go client.WritePump()
	go client.ReadPump()
}
