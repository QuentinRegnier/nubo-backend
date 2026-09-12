package websocket

import (
	"context"
	"encoding/json"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 30 * time.Second
	maxMessageSize = 1024
)

// WSIncomingMessage est l'enveloppe standard pour les messages envoyés par la PWA au serveur
type WSIncomingMessage struct {
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"` // json.RawMessage permet de différer le décodage
}

// TypingPayload est la structure attendue quand event_type = "typing.started" ou "typing.stopped"
type TypingPayload struct {
	ConversationID int64 `json:"conversation_id"`
}

type Client struct {
	Hub      *Hub
	Conn     *websocket.Conn
	UserID   int64
	DeviceID string // NOUVEAU : Essentiel pour cibler la bonne session Ratchet en RAM
	Send     chan []byte
}

// ReadPump écoute les événements entrants du client (PONG et événements temps réel)
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister <- c
		_ = c.Conn.Close()
		// Pas besoin d'appeler de fonction pour dire "offline" !
		// Si le client crashe ou quitte, les pings JSON s'arrêtent,
		// et Redis fera expirer le TTL de 90s tout seul. Magique.
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))

	// Optionnel : Tu peux garder le SetPongHandler si tu veux supporter les deux méthodes
	// (Pings natifs ET Pings JSON). C'est ce que je ferais en tant que Senior pour être ultra robuste.
	c.Conn.SetPongHandler(func(string) error {
		_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		// Si un client envoie un vrai ping natif, on le marque aussi en ligne
		_ = cache_service.MarkUserOnline(context.Background(), c.UserID)
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			break // Déconnexion naturelle ou Timeout
		}

		// Dès qu'on reçoit un message (n'importe lequel : typing, message.create, ou ping),
		// on prolonge le bail de présence. L'activité prouve la présence.
		_ = cache_service.MarkUserOnline(context.Background(), c.UserID)

		// On envoie le message brut au routeur !
		c.Route(message)
	}
}

// handleTyping vérifie la sécurité et broadcast l'événement de frappe en O(1)
func (c *Client) handleTyping(eventType string, convID int64) {
	ctx := context.Background()

	// SÉCURITÉ ZERO-TRUST : Le client WebSocket est peut-être malveillant.
	// On s'assure qu'il est bien membre de la conversation qu'il prétend cibler.
	// Cette requête est ultra-rapide (L1 Speed Cache Object)[cite: 16].
	mem, err := security_service.LeftMember(ctx, convID, c.UserID)
	if err != nil || mem.Role < 0 {
		return // Accès refusé, l'utilisateur a été banni ou n'est pas membre. On drop silencieusement.
	}

	// Préparation du payload à redistribuer aux autres membres
	broadcastPayload := map[string]any{
		"conversation_id": convID,
		"user_id":         c.UserID,
	}

	// Délégation au Pub/Sub : Cela va propager l'info dans les canaux Redis des autres utilisateurs[cite: 16].
	_ = realtime_service.BroadcastToConversation(ctx, convID, eventType, broadcastPayload)
}

// WritePump est la seule goroutine qui écrit physiquement sur la connexion TCP
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)

			n := len(c.Send)
			for i := 0; i < n; i++ {
				_, _ = w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
