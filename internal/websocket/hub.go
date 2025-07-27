package websocket

import (
	"context"
	"log"
	"sync"

	"github.com/QuentinRegnier/nubo-backend/internal/cache"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
)

type Client struct {
	conn *websocket.Conn
	send chan []byte
}

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.Mutex

	pubsub  *redis.PubSub
	channel string
}

func NewHub() *Hub {
	h := &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		channel:    "nubo-websocket",
	}

	h.pubsub = cache.Rdb.Subscribe(context.Background(), h.channel)
	go h.listenPubSub()

	return h
}

func (h *Hub) listenPubSub() {
	ch := h.pubsub.Channel()
	for msg := range ch {
		h.mu.Lock()
		for client := range h.clients {
			select {
			case client.send <- []byte(msg.Payload):
			default:
				close(client.send)
				delete(h.clients, client)
			}
		}
		h.mu.Unlock()
	}
}

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
			// Publie sur Redis
			err := cache.Rdb.Publish(context.Background(), h.channel, message).Err()
			if err != nil {
				log.Println("Redis publish error:", err)
			}
		}
	}
}

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

		// Envoie le message aux autres clients
		hub.broadcast <- msg
	}
}

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
