package websocket

import (
	"context"
	"encoding/json"
	"log"

	"github.com/QuentinRegnier/nubo-backend/internal/api/websocket/ws_handlers"
)

// Route intercepte le message brut, l'oriente et gère la réponse.
func (c *Client) Route(message []byte) {
	var req WSRequest
	if err := json.Unmarshal(message, &req); err != nil {
		log.Printf("WS Route Error: Payload illisible: %v", err)
		return
	}

	// Contexte vide car le WebSocket est une boucle infinie, on n'a pas de Request.Context() HTTP
	ctx := context.Background()
	var resData any
	var err error

	// Le Grand Switch (Remplace ton HTTP routes.go)
	switch req.Action {

	// --- TYPING (Volatil) ---
	case "typing.started":
		err = ws_handlers.HandleTyping(ctx, c.UserID, req.Payload, true)
	case "typing.stopped":
		err = ws_handlers.HandleTyping(ctx, c.UserID, req.Payload, false)

	// --- CONVERSATIONS ---
	case "conversation.read":
		err = ws_handlers.HandleReadReceipt(ctx, c.UserID, req.Payload)

	// --- MESSAGES ---
	case "message.create":
		resData, err = ws_handlers.HandleCreateMessage(ctx, c.UserID, req.Payload)
	case "message.update":
		err = ws_handlers.HandleUpdateMessage(ctx, c.UserID, req.Payload)
	case "message.delete":
		err = ws_handlers.HandleDeleteMessage(ctx, c.UserID, req.Payload)
	case "message.react":
		err = ws_handlers.HandleReactMessage(ctx, c.UserID, req.Payload)
	case "message.unreact":
		err = ws_handlers.HandleUnreactMessage(ctx, c.UserID, req.Payload)

	default:
		c.SendError(req.RequestID, "Action non reconnue")
		return
	}

	// Gestion de la réponse à renvoyer au client
	if err != nil {
		c.SendError(req.RequestID, err.Error())
	} else {
		c.SendSuccess(req.RequestID, resData)
	}
}

// Helpers pour standardiser les retours
func (c *Client) SendSuccess(reqID string, data any) {
	resp := WSResponse{EventType: "response", RequestID: reqID, Status: "success", Data: data}
	b, _ := json.Marshal(resp)
	c.Send <- b
}

func (c *Client) SendError(reqID string, errStr string) {
	resp := WSResponse{EventType: "response", RequestID: reqID, Status: "error", Error: errStr}
	b, _ := json.Marshal(resp)
	c.Send <- b
}
