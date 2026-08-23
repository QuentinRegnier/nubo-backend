package worker

import (
	"context"
	"encoding/json"
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
	log.Println("📱 Démarrage du Worker Firebase Cloud Messaging...")

	// 1. Initialisation SANS "WithCredentialsFile" (DÉPRÉCIÉ)
	// Firebase lira nativement la variable d'environnement GOOGLE_APPLICATION_CREDENTIALS
	app, err := firebase.NewApp(ctx, nil)
	if err != nil {
		log.Printf("⚠️ Firebase non initialisé : %v", err)
		return
	}

	fcmClient, err = app.Messaging(ctx)
	if err != nil {
		log.Printf("⚠️ Erreur Client Firebase Messaging : %v", err)
		return
	}

	// 2. Boucle de consommation (Zéro CPU via BLPOP)
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
	// 1. On récupère TOUS les identifiants d'appareils via la cascade L1 -> L2 -> L3 (DDD)
	fids, err := cache_service.GetFirebaseInstallationIDsCascade(ctx, job.UserID)
	if err != nil || len(fids) == 0 {
		return // L'utilisateur n'a aucun appareil connecté
	}

	// 2. Conversion du Payload dynamique en string JSON pour FCM
	payloadBytes, _ := json.Marshal(job.Payload)
	payloadStr := string(payloadBytes)

	// 3. Construction du message Multicast (Le standard officiel pour le multi-session)
	message := &messaging.MulticastMessage{
		Fids: fids,
		Data: map[string]string{
			"event_type": job.EventType,
			"payload":    payloadStr,
		},
		Notification: &messaging.Notification{
			Title: "Nubo",
			Body:  "Nouvelle activité",
		},
		Android: &messaging.AndroidConfig{
			CollapseKey: job.EventType, // Le Batching natif d'Android
			Priority:    "high",
		},
		APNS: &messaging.APNSConfig{
			Headers: map[string]string{
				"apns-collapse-id": job.EventType, // Le Batching natif d'Apple
			},
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					MutableContent: true,
				},
			},
		},
		Webpush: &messaging.WebpushConfig{
			Headers: map[string]string{
				"Topic": job.EventType, // Le Batching pour navigateurs (Chrome/Firefox)
			},
		},
	}

	// 4. Envoi physique chez Google/Apple avec la méthode non dépréciée
	br, err := fcmClient.SendEachForMulticast(ctx, message)
	if err != nil {
		log.Printf("❌ Erreur Firebase: %v", err)
	} else if br.FailureCount > 0 {
		log.Printf("⚠️ Firebase a rejeté %d/%d tokens pour l'utilisateur %d", br.FailureCount, len(fids), job.UserID)
	}
}
