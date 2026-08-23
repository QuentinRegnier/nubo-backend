package websocket

import "encoding/json"

// WSRequest est l'enveloppe envoyée par la PWA vers le Serveur
type WSRequest struct {
	RequestID string          `json:"request_id"` // Généré par le front (ex: UUID) pour faire correspondre la réponse
	Action    string          `json:"action"`     // Ex: "message.create", "typing.started"
	Payload   json.RawMessage `json:"payload"`    // Le contenu dynamique brut
}

// WSResponse est l'accusé de réception envoyé par le Serveur vers la PWA
type WSResponse struct {
	EventType string `json:"event_type"`           // Sera toujours "response"
	RequestID string `json:"request_id"`           // L'ID de la requête d'origine
	Status    string `json:"status"`               // "success" ou "error"
	Data      any    `json:"data,omitempty"`       // Les données retournées (ex: l'ID du message créé)
	Error     string `json:"nubo_error,omitempty"` // Le message d'erreur si échec
}
