package websocket

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var hub *Hub

func InitHub() {
	if hub == nil {
		hub = NewHub()
		go hub.Run()
	}
}

func GetHub() *Hub {
	return hub
}

func WSHandler(c *gin.Context) {
	userID := c.GetString("userID")
	log.Println("Utilisateur connecté (userID):", userID)

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	client := &Client{
		conn: conn,
		send: make(chan []byte, 256),
	}

	hub.register <- client

	go client.WritePump()
	client.ReadPump(hub)
}
