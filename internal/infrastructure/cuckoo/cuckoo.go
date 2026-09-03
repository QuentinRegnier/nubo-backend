package cuckoo

import (
	"context"
	"encoding/json"

	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
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
	logger.Log.Info().Msg("Initialisation du Cuckoo Filter...")

	// 1. Création du filtre (Capacité 1M, peut être ajusté)
	GlobalCuckoo = cuckoo.NewFilter(1000000)

	// 2. Warm-up : Chargement des données existantes depuis Postgres (Pur DDD)
	logger.Log.Info().Msg("Chargement des données Postgres dans le Cuckoo Filter...")

	ctx := context.Background()
	identifiers, err := postgres.FuncLoadAllUserIdentifiers(ctx)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Erreur critique init Cuckoo (SQL via Repository)")
	}

	count := 0
	for _, idents := range identifiers {
		if idents.Username != nil && *idents.Username != "" {
			GlobalCuckoo.Insert([]byte("username:" + *idents.Username))
		}
		if idents.Email != nil && *idents.Email != "" {
			GlobalCuckoo.Insert([]byte("email:" + *idents.Email))
		}
		if idents.Phone != nil && *idents.Phone != "" {
			GlobalCuckoo.Insert([]byte("phone:" + *idents.Phone))
		}
		count++
	}
	logger.Log.Info().Int("count", count).Msg("Cuckoo Filter chargé avec des utilisateurs (x3 clés).")

	// 3. Lancement de la synchro inter-serveurs (Flux Redis)
	go startCuckooSync()
}

// startCuckooSync écoute le flux Redis pour mettre à jour le filtre local
func startCuckooSync() {
	// Utilisation pure DDD : la Collection encapsule tout (canal, nom, client)
	msgChan, cancel := redisgo.CuckooSync.SubscribeFlux(context.Background())
	defer cancel()

	logger.Log.Info().Msg("Cuckoo Sync : écoute du flux Redis activée.")

	for payload := range msgChan {
		var msg CuckooMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			logger.Log.Error().Err(err).Msg("Erreur décodage message Cuckoo")
			continue
		}

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
		Key:    field + ":" + value, // Concaténation pure et simple
	}

	data, _ := json.Marshal(msg)

	// Utilisation pure DDD : Zéro exposition de l'infrastructure sous-jacente
	if err := redisgo.CuckooSync.PushFlux(context.Background(), data); err != nil {
		logger.Log.Error().Err(err).Msg("Erreur Broadcast Cuckoo")
	}
}
