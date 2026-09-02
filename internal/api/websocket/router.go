package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/api/websocket/ws_handlers"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/security"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// Route intercepte le message brut, l'oriente et gère la réponse avec un bouclier Zero-Trust.
func (c *Client) Route(message []byte) {
	var req WSRequest
	if err := json.Unmarshal(message, &req); err != nil {
		logger.Log.Error().Err(err).Msg("WS Route Error: Payload illisible")
		return
	}

	ctx := context.Background()

	// =========================================================================
	// BOUCLIER DE SÉCURITÉ ZERO-TRUST (HMAC & Anti-Rejeu)
	// =========================================================================

	// 1. Anti-Rejeu (Timestamp)
	tsInt, err := strconv.ParseInt(req.Timestamp, 10, 64)
	if err != nil {
		c.SendError(req.RequestID, "Timestamp invalide")
		return
	}
	now := time.Now().Unix()
	if math.Abs(float64(now-tsInt)) > variables.ToleranceTimeSeconds {
		c.SendError(req.RequestID, "Trame expirée (Anti-Rejeu)")
		return
	}

	// 2. Récupération instantanée de la session Ratchet en RAM (Speed Cache O(1))
	// Cela permet de supporter la rotation de clé HTTP sans couper le WebSocket !
	session, err := cache_service.LoadSessionFromCache(ctx, c.UserID, c.DeviceID, "")
	if err != nil || session.ID == 0 {
		c.SendError(req.RequestID, "Session invalide ou expirée")
		return
	}

	// 3. Validation de la signature HMAC (Action|Timestamp|Payload)
	stringToSign := fmt.Sprintf("%s|%s|%s", req.Action, req.Timestamp, string(req.Payload))

	isValid := security.CheckHMAC(stringToSign, session.CurrentSecret, req.Signature)

	// Tolérance de rotation de clé (Exactement comme en HTTP)
	if !isValid && session.LastSecret != "" && !session.ToleranceTime.IsZero() && time.Now().Before(session.ToleranceTime) {
		isValid = security.CheckHMAC(stringToSign, session.LastSecret, req.Signature)
	}

	if !isValid {
		c.SendError(req.RequestID, "Signature HMAC invalide")
		return
	}
	// =========================================================================

	var resData any
	var routeErr error

	// Le Grand Switch (Remplace ton HTTP routes.go)
	switch req.Action {
	// --- TYPING (Volatil) ---
	case "typing.started":
		routeErr = ws_handlers.HandleTyping(ctx, c.UserID, req.Payload, true)
	case "typing.stopped":
		routeErr = ws_handlers.HandleTyping(ctx, c.UserID, req.Payload, false)

	// --- CONVERSATIONS ---
	case "conversation.read":
		routeErr = ws_handlers.HandleReadReceipt(ctx, c.UserID, req.Payload)

	// --- MESSAGES ---
	case "message.create":
		resData, routeErr = ws_handlers.HandleCreateMessage(ctx, c.UserID, req.Payload)
	case "message.update":
		routeErr = ws_handlers.HandleUpdateMessage(ctx, c.UserID, req.Payload)
	case "message.delete":
		routeErr = ws_handlers.HandleDeleteMessage(ctx, c.UserID, req.Payload)
	case "message.react":
		routeErr = ws_handlers.HandleReactMessage(ctx, c.UserID, req.Payload)
	case "message.unreact":
		routeErr = ws_handlers.HandleUnreactMessage(ctx, c.UserID, req.Payload)

	default:
		c.SendError(req.RequestID, "Action non reconnue")
		return
	}

	// Gestion de la réponse à renvoyer au client
	if routeErr != nil {
		c.SendError(req.RequestID, routeErr.Error())
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
