package pkg

import (
	"errors"
	"sync"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

var generator *Node // La variable globale privée (Singleton)

// InitSnowflake : Tu l'appelles une fois au démarrage dans le main
func InitSnowflake(nodeID int64) error {
	var err error
	generator, err = NewNode(nodeID)
	return err
}

// GenerateID : LA fameuse fonction simple que tu voulais
func GenerateID() int64 {
	return generator.Generate()
}

// Node est la structure qui génère les IDs
type Node struct {
	mu        sync.Mutex
	timestamp int64
	nodeID    int64
	step      int64
}

// NewNode crée une nouvelle instance de générateur Snowflake.
// nodeID : Un identifiant unique pour ce serveur (entre 0 et 1023).
func NewNode(nodeID int64) (*Node, error) {
	if nodeID < 0 || nodeID > variables.NodeMax {
		return nil, errors.New("node ID must be between 0 and 1023")
	}

	return &Node{
		timestamp: 0,
		nodeID:    nodeID,
		step:      0,
	}, nil
}

// Generate crée et retourne un nouvel ID unique (int64).
func (n *Node) Generate() int64 {
	n.mu.Lock()         // Verrouille pour éviter que deux goroutines accèdent en même temps
	defer n.mu.Unlock() // Déverrouille à la fin de la fonction

	now := time.Now().UnixMilli() // Temps actuel en millisecondes

	if now < n.timestamp {
		// Protection contre le changement d'heure système (NTP drift)
		// On attend que l'horloge rattrape le dernier timestamp
		for now <= n.timestamp {
			now = time.Now().UnixMilli()
		}
	}

	if n.timestamp == now {
		// Si on est dans la même milliseconde, on incrémente la séquence
		n.step = (n.step + 1) & variables.StepMax

		// Si la séquence déborde (plus de 4096 IDs dans la même ms), on attend la ms suivante
		if n.step == 0 {
			for now <= n.timestamp {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		// Nouvelle milliseconde : on remet la séquence à zéro
		n.step = 0
	}

	n.timestamp = now

	// Construction de l'ID final par opérations binaires (Bitwise OR)
	// 1. (now - epoch) << timeShift : On place le temps tout à gauche
	// 2. (nodeID << nodeShift)      : On place l'ID du noeud au milieu
	// 3. (n.step)                   : On place la séquence à la fin
	id := ((now - variables.Epoch) << variables.TimeShift) | (n.nodeID << variables.NodeShift) | (n.step)

	return id
}
