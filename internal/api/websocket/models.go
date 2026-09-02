package websocket

import "encoding/json"

// WSRequest est l'enveloppe envoyée par la PWA vers le Serveur
type WSRequest struct {
	RequestID string          `json:"request_id"`
	Action    string          `json:"action"`
	Payload   json.RawMessage `json:"payload"`
	Signature string          `json:"x_signature"` // NOUVEAU: HMAC
	Timestamp string          `json:"x_timestamp"` // NOUVEAU: Anti-rejeu
}

// WSResponse est l'accusé de réception envoyé par le Serveur vers la PWA
type WSResponse struct {
	EventType string `json:"event_type"`
	RequestID string `json:"request_id"`
	Status    string `json:"status"`
	Data      any    `json:"data,omitempty"`
	Error     string `json:"nubo_error,omitempty"`
}
