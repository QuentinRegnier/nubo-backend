package cuckoo

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	redisgo "github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	cuckoo "github.com/seiflotfy/cuckoofilter"
)

// GlobalCuckoo est l'instance unique du filtre (Singleton)
var GlobalCuckoo *cuckoo.Filter

// Constantes pour le flux de synchro
const (
	CuckooChannel = "cuckoo-sync"
	ActionAdd     = "ADD"
	ActionDel     = "DEL"
)

// CuckooMessage définit le format des messages envoyés dans le Flux Redis
type CuckooMessage struct {
	Action string // "ADD" ou "DEL"
	Key    string // ex: "username:toto"
}

// InitCuckooFilter initialise le filtre, charge les données de Postgres et lance l'écoute Redis
func InitCuckooFilter() {
	log.Println("🔒 Initialisation du Cuckoo Filter...")

	// 1. Création du filtre (Capacité 1M, peut être ajusté)
	GlobalCuckoo = cuckoo.NewFilter(1000000)

	// 2. Warm-up : Chargement des données existantes depuis Postgres
	// On récupère TOUS les champs uniques (username, email, phone) pour éviter les faux négatifs au démarrage
	log.Println("🔄 Chargement des données Postgres dans le Cuckoo Filter...")
	rows, err := postgres.PostgresDB.Query("SELECT username, email, phone FROM auth.users")
	if err != nil {
		log.Fatalf("❌ Erreur critique init Cuckoo (SQL): %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var u, e, p string
		if err := rows.Scan(&u, &e, &p); err == nil {
			if u != "" {
				GlobalCuckoo.Insert([]byte("username:" + u))
			}
			if e != "" {
				GlobalCuckoo.Insert([]byte("email:" + e))
			}
			if p != "" {
				GlobalCuckoo.Insert([]byte("phone:" + p))
			}
			count++
		}
	}
	log.Printf("✅ Cuckoo Filter chargé avec %d utilisateurs (x3 clés).", count)

	// 3. Lancement de la synchro inter-serveurs (Flux Redis)
	go startCuckooSync()
}

// startCuckooSync écoute le flux Redis pour mettre à jour le filtre local
func startCuckooSync() {
	// Utilisation de TA fonction SubscribeFlux
	// On s'abonne au canal "cuckoo-sync"
	msgChan, cancel := redisgo.SubscribeFlux(redis.Rdb, CuckooChannel)
	defer cancel()

	log.Println("📡 Cuckoo Sync : Écoute du flux Redis activée.")

	for payload := range msgChan {
		var msg CuckooMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			log.Printf("⚠️ Erreur décodage message Cuckoo: %v", err)
			continue
		}

		// Mise à jour du filtre local en RAM
		if msg.Action == ActionAdd {
			GlobalCuckoo.Insert([]byte(msg.Key))
		} else if msg.Action == ActionDel {
			GlobalCuckoo.Delete([]byte(msg.Key))
		}
	}
}

// BroadcastCuckooUpdate envoie un signal aux autres serveurs via Redis
func BroadcastCuckooUpdate(action, field, value string) {
	msg := CuckooMessage{
		Action: action,
		Key:    fmt.Sprintf("%s:%s", field, value),
	}

	data, _ := json.Marshal(msg)

	// Utilisation de TA fonction PushFluxWithTTL
	// On met un TTL court car c'est de l'événementiel pur
	msgID := fmt.Sprintf("%d", time.Now().UnixNano())
	err := redisgo.PushFluxWithTTL(redis.Rdb, CuckooChannel, msgID, data, 5*time.Second)

	if err != nil {
		log.Printf("⚠️ Erreur Broadcast Cuckoo: %v", err)
	}
}
