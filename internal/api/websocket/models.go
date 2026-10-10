package websocket

import "encoding/json"

// wsRequest est l'enveloppe envoyée par la PWA vers le Serveur
type wsRequest struct {
	RequestID string          `json:"request_id"`
	Action    string          `json:"action"`
	Payload   json.RawMessage `json:"payload"`
	Signature string          `json:"x_signature"` // NOUVEAU: HMAC
	Timestamp string          `json:"x_timestamp"` // NOUVEAU: Anti-rejeu
}

// wsResponse est l'accusé de réception envoyé par le Serveur vers la PWA
type wsResponse struct {
	EventType string `json:"event_type"`
	RequestID string `json:"request_id"`
	Status    string `json:"status"`
	Data      any    `json:"data,omitempty"`
	Error     string `json:"numan_error,omitempty"`
}
