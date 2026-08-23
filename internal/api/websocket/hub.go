package websocket

import (
	"context"
	"fmt"
	"strings"

	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/go-redis/redis/v8"
)

// BroadcastMessage est l'enveloppe interne pour transférer l'écoute Redis vers la boucle locale
type BroadcastMessage struct {
	ChannelType string // "user" ou "community"
	TargetID    int64
	Payload     []byte
}

type CommunitySubscription struct {
	Client      *Client
	CommunityID int64
}

type Hub struct {
	// Mode "Fan-out" (Multi-Device)
	Clients map[int64]map[*Client]bool

	// Mode "Twitch" (Duplication RAM)
	Communities map[int64]map[*Client]bool

	Register           chan *Client
	Unregister         chan *Client
	SubscribeCommunity chan CommunitySubscription

	// SEULE voie autorisée pour ordonner l'envoi d'un message (Anti Race-Condition)
	BroadcastRoute chan BroadcastMessage

	PubSub *redis.PubSub
}

var GlobalHub *Hub

func InitHub() {
	GlobalHub = &Hub{
		Clients:            make(map[int64]map[*Client]bool),
		Communities:        make(map[int64]map[*Client]bool),
		Register:           make(chan *Client),
		Unregister:         make(chan *Client),
		SubscribeCommunity: make(chan CommunitySubscription),
		BroadcastRoute:     make(chan BroadcastMessage, 2048), // Buffer haute capacité
		PubSub:             redisgo.Rdb.Subscribe(context.Background()),
	}

	go GlobalHub.Run()
	go GlobalHub.ListenRedis()
}

// ListenRedis capte les événements du réseau global et les pousse dans le sas d'attente
func (h *Hub) ListenRedis() {
	ch := h.PubSub.Channel()
	for msg := range ch {
		parts := strings.Split(msg.Channel, ":")
		if len(parts) != 3 {
			continue
		}

		channelType := parts[1] // "user" ou "community"
		var targetID int64
		_, _ = fmt.Sscanf(parts[2], "%d", &targetID)

		h.BroadcastRoute <- BroadcastMessage{
			ChannelType: channelType,
			TargetID:    targetID,
			Payload:     []byte(msg.Payload),
		}
	}
}

// Run est l'unique boucle autorisée à muter les maps et écrire dans client.Send
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			if _, ok := h.Clients[client.UserID]; !ok {
				h.Clients[client.UserID] = make(map[*Client]bool)
				// Abonnement dynamique au cluster
				channel := fmt.Sprintf("channel:user:%d", client.UserID)
				_ = h.PubSub.Subscribe(context.Background(), channel)
			}
			h.Clients[client.UserID][client] = true

		case client := <-h.Unregister:
			if connections, ok := h.Clients[client.UserID]; ok {
				if _, ok := connections[client]; ok {
					delete(connections, client)
					close(client.Send)

					if len(connections) == 0 {
						delete(h.Clients, client.UserID)
						// Désabonnement réseau pour économiser le CPU Redis
						channel := fmt.Sprintf("channel:user:%d", client.UserID)
						_ = h.PubSub.Unsubscribe(context.Background(), channel)
					}
				}
			}

		case sub := <-h.SubscribeCommunity:
			if _, ok := h.Communities[sub.CommunityID]; !ok {
				h.Communities[sub.CommunityID] = make(map[*Client]bool)
				channel := fmt.Sprintf("channel:community:%d", sub.CommunityID)
				_ = h.PubSub.Subscribe(context.Background(), channel)
			}
			h.Communities[sub.CommunityID][sub.Client] = true

		case msg := <-h.BroadcastRoute:
			if msg.ChannelType == "user" {
				if connections, ok := h.Clients[msg.TargetID]; ok {
					for client := range connections {
						select {
						case client.Send <- msg.Payload:
						default:
							// Le client a un buffer plein (plantage réseau), on le coupe
							close(client.Send)
							delete(connections, client)
						}
					}
				}
			} else if msg.ChannelType == "community" {
				if connections, ok := h.Communities[msg.TargetID]; ok {
					for client := range connections {
						select {
						case client.Send <- msg.Payload:
						default:
							close(client.Send)
							delete(connections, client)
						}
					}
				}
			}
		}
	}
}
