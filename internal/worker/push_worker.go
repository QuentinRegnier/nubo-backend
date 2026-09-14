package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
)

var fcmClient *messaging.Client

type PushJob struct {
	UserID    int64  `json:"user_id"`
	EventType string `json:"event_type"`
	Payload   any    `json:"payload"`
}

// StartPushNotificationWorker initialise la connexion à Google et écoute Redis
func StartPushNotificationWorker(ctx context.Context) {
	log.Println("Démarrage du Worker Firebase Cloud Messaging...")

	app, err := firebase.NewApp(ctx, nil)
	if err != nil {
		log.Printf("Erreur Firebase non initialisé : %v", err)
		return
	}

	fcmClient, err = app.Messaging(ctx)
	if err != nil {
		log.Printf("Erreur Client Firebase Messaging : %v", err)
		return
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				res, err := redisgo.Rdb.BLPop(ctx, 2*time.Second, "worker:queue:firebase").Result()
				if err != nil {
					continue // File vide
				}
				if len(res) == 2 {
					var job PushJob
					if err := json.Unmarshal([]byte(res[1]), &job); err == nil {
						processFirebaseJob(ctx, job)
					}
				}
			}
		}
	}()
}

func processFirebaseJob(ctx context.Context, job PushJob) {
	// 1. On récupère TOUS les identifiants d'appareils via la cascade L1 -> L2 -> L3
	fids, err := cache_service.GetFirebaseInstallationIDsCascade(ctx, job.UserID)
	if err != nil || len(fids) == 0 {
		return // L'utilisateur n'a aucun appareil connecté
	}

	// 2. Conversion du Payload dynamique en string JSON pour FCM
	payloadBytes, _ := json.Marshal(job.Payload)
	payloadStr := string(payloadBytes)

	// === NOUVEAU : FORMATAGE DYNAMIQUE (Titres, Corps et Sons) ===
	title := "Nubo"
	body := "Nouvelle notification"
	soundIOS := "default"
	channelAndroid := "default_channel"

	// On parse le payload pour en extraire les informations (pseudo, contenu)
	var payloadMap map[string]interface{}
	_ = json.Unmarshal(payloadBytes, &payloadMap)

	switch job.EventType {
	case "message.new":
		if sender, ok := payloadMap["sender_username"].(string); ok && sender != "" {
			title = fmt.Sprintf("Nouveau message de %s", sender)
		} else {
			title = "Nouveau message"
		}
		if content, ok := payloadMap["content"].(string); ok && content != "" {
			body = content
		}

	case "message.mention":
		if sender, ok := payloadMap["sender_username"].(string); ok && sender != "" {
			title = fmt.Sprintf("@%s vous a mentionné(e)", sender)
		} else {
			title = "Vous avez été mentionné(e)"
		}
		if content, ok := payloadMap["content"].(string); ok && content != "" {
			body = content
		}
		// Personnalisation des alertes pour les mentions
		soundIOS = "mention.wav"            // Fichier audio à inclure dans Xcode
		channelAndroid = "mentions_channel" // Channel ID à configurer sur Android (Flutter)
	}

	// 3. Construction du message Multicast (Le standard officiel pour le multi-session)
	message := &messaging.MulticastMessage{
		Fids: fids,
		Data: map[string]string{
			"event_type": job.EventType,
			"payload":    payloadStr,
		},
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Android: &messaging.AndroidConfig{
			CollapseKey: job.EventType,
			Priority:    "high",
			Notification: &messaging.AndroidNotification{
				ChannelID: channelAndroid, // Gestion des sons/priorités native sur Android
			},
		},
		APNS: &messaging.APNSConfig{
			Headers: map[string]string{
				"apns-collapse-id": job.EventType,
			},
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					MutableContent: true,
					Sound:          soundIOS, // Son spécifique sur iOS
				},
			},
		},
		Webpush: &messaging.WebpushConfig{
			Headers: map[string]string{
				"Topic": job.EventType,
			},
		},
	}

	// 4. Envoi physique chez Google/Apple
	br, err := fcmClient.SendEachForMulticast(ctx, message)
	if err != nil {
		log.Printf("Erreur Firebase: %v", err)
	} else if br.FailureCount > 0 {
		log.Printf("Firebase a rejeté %d/%d tokens pour l'utilisateur %d", br.FailureCount, len(fids), job.UserID)
	}
}
