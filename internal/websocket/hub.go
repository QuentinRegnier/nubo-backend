package websocket

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"sync"

	"github.com/QuentinRegnier/nubo-backend/internal/cache"
	"github.com/gorilla/websocket"
)

// generateMessageID crée un ID unique pour chaque message
func generateMessageID() string {
	b := make([]byte, 8) // 8 octets → 16 caractères hex
	if _, err := rand.Read(b); err != nil {
		return "msg-fallback" // fallback si erreur improbable
	}
	return hex.EncodeToString(b)
}

// ---------------- Clients ----------------

type Client struct {
	conn *websocket.Conn
	send chan []byte
}

// ---------------- Hub ----------------

type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	mu         sync.Mutex

	channel string
}

// NewHub crée un nouveau Hub et lance l'écoute du flux Redis
func NewHub() *Hub {
	h := &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte),
		channel:    "nubo-websocket",
	}

	// Utilise la fonction SubscribeFlux pour recevoir les messages
	go h.listenFlux()
	return h
}

// listenFlux s'abonne au flux Redis et distribue les messages aux clients
func (h *Hub) listenFlux() {
	ch, cancel := cache.SubscribeFlux(cache.Rdb, h.channel)
	defer cancel()

	for msg := range ch {
		h.mu.Lock()
		for client := range h.clients {
			select {
			case client.send <- msg:
			default:
				close(client.send)
				delete(h.clients, client)
			}
		}
		h.mu.Unlock()
	}
}

// Run démarre la boucle principale du hub pour gérer l'inscription/désinscription et la diffusion
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Println("Client registered")

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				log.Println("Client unregistered")
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			// Publie le message sur le flux Redis avec TTL individuel (ex: 1s)
			messageID := generateMessageID() // fonction pour créer un ID unique
			err := cache.PushFluxWithTTL(cache.Rdb, h.channel, messageID, message, cache.DefaultFluxTTL)
			if err != nil {
				log.Println("Erreur PushFluxWithTTL:", err)
			}
		}
	}
}

// ---------------- Clients WS ----------------

// ReadPump lit les messages d’un client et les envoie au hub
func (c *Client) ReadPump(hub *Hub) {
	defer func() {
		hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break
		}

		// TODO: sauvegarder msg en base (Postgres/Mongo)

		// Envoie le message aux autres clients via le hub
		hub.broadcast <- msg
	}
}

// WritePump envoie les messages du hub au client
func (c *Client) WritePump() {
	for msg := range c.send {
		err := c.conn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			log.Println("Write error:", err)
			break
		}
	}
	c.conn.Close()
}
