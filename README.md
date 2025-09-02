# NuboBackend
## ✅ Objectif final

Un backend nommé `nubo-backend` avec :

- API REST (auth, CRUD de posts/messages, etc.)
- WebSocket (temps réel)
- Redis (cache/session)
- PostgreSQL (stockage permanent)
- MongoDB (stockage temporaire 30 jours)
- Docker (zéro installation locale, sauf Docker)

---

## 🧱 PRÉREQUIS (macOS)

Avant tout, installe :

- [Docker Desktop pour Mac](https://www.docker.com/products/docker-desktop/)
  - Ouvre-le une fois installé
- Go (Golang) *(facultatif si tu build tout dans Docker, mais conseillé pour dev local)*  
  `brew install go`
- Git *(souvent déjà présent)*  
  `git --version`
- (Optionnel) Un bon éditeur :
  - [VS Code](https://code.visualstudio.com/)
    - Extensions recommandées : **Go**, **Docker**, **GitLens**

---

## 🗂️ STRUCTURE DU PROJET

```
nubo-backend/
├── docker-compose.yml
├── Dockerfile
├── .env
├── go.mod
├── cmd/
│   └── main.go
├── internal/
│   ├── api/         ← Handlers REST
│   ├── websocket/   ← Serveur WebSocket
│   ├── db/          ← Connexion PostgreSQL & Mongo
│   ├── cache/       ← Connexion Redis
│   └── models/      ← Structs & logique métier
└── README.md
```

---

## I. 🔨 ÉTAPES DE DÉVELOPPEMENT GO/DOCKER

### 1. Initialiser ton projet Go

```bash
mkdir nubo-backend && cd nubo-backend
go mod init github.com/tonuser/nubo-backend
```

> Remplace `tonuser` par ton pseudo GitHub

---

### 2. Créer les fichiers essentiels

```bash
touch Dockerfile docker-compose.yml .env README.md
mkdir -p cmd internal/{api,websocket,db,cache,models}
touch cmd/main.go
```

---

### 3. Ajouter les dépendances Go

Installe les libs de base avec :

```bash
go get github.com/gin-gonic/gin              # REST API
go get github.com/go-redis/redis/v8          # Redis
go get go.mongodb.org/mongo-driver/mongo     # MongoDB
go get github.com/jackc/pgx/v5               # PostgreSQL
go get github.com/gorilla/websocket          # WebSocket
```

---

### 4. Écrire ton `main.go`

```go
package main

import (
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	r.Run(":8080")
}
```

---

### 5. Créer le `Dockerfile`

```Dockerfile
FROM golang:1.24.5

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

RUN go build -o nubo cmd/main.go

EXPOSE 8080

CMD ["./nubo"]
```

---

### 6. Créer le `docker-compose.yml`

```yaml
services:
  api:
    build: .
    container_name: nubo_api
    ports:
      - "8080:8080"
    env_file:
      - .env
    depends_on:
      - redis
      - postgres
      - mongo
    restart: always

  redis:
    image: redis:7
    container_name: nubo_redis
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data

  postgres:
    image: postgres:15
    container_name: nubo_postgres
    environment:
      POSTGRES_USER: nubo
      POSTGRES_PASSWORD: nubo
      POSTGRES_DB: nubo
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

  mongo:
    image: mongo:7
    container_name: nubo_mongo
    ports:
      - "27017:27017"
    volumes:
      - mongo_data:/data/db

volumes:
  redis_data:
  postgres_data:
  mongo_data:
```

---

### 7. Lancer ton environnement

```bash
docker-compose up --build
```

Puis va sur [http://localhost:8080/ping](http://localhost:8080/ping) → tu dois voir :

```json
{ "message": "pong" }
```

---

## II. 🌐 Création du repo GitHub

- Créer un repo vide (sur github.com) :
- Nom : nubo-backend
- Visibilité : Privé ou Public, à toi de voir
- Puis en terminal :
```bash
git init
git remote add origin https://github.com/TON_USER/nubo-backend.git
git add .
git commit -m "Initial commit"
git branch -M main
git push -u origin main
```

---

## III. 🛠️ Plan d’action clair (prochaine phase go)

✅ Étape 1 — Ajouter les routes REST : Login, Signup, Post
✅ Étape 2 — Ajouter le WebSocket en Go
✅ Étape 3 — Brancher PostgreSQL, MongoDB, Redis
✅ Étape 4 — Ajouter l’authentification JWT
✅ Étape 5 — Déployer sur un serveur Linux (Docker Compose + .env)
✅ ÉTAPE 1 — Créer les routes REST de base

---

### 🎯 Objectif
**Créer des routes :**

- `POST /signup` → créer un utilisateur
- `POST /login` → connecter (renvoyer JWT, à faire en Étape 4)
- `GET /posts` → récupérer les posts
- `POST /posts` → créer un post

### 📁 Organisation recommandée
Fichier : `internal/api/routes.go`
```go
package api

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func SetupRoutes(r *gin.Engine) {
	r.POST("/signup", SignUpHandler)
	r.POST("/login", LoginHandler)

	r.GET("/posts", GetPostsHandler)
	r.POST("/posts", CreatePostHandler)
}

func SignUpHandler(c *gin.Context) {
	// TODO: lire JSON, enregistrer utilisateur
	c.JSON(http.StatusOK, gin.H{"message": "signup ok"})
}

func LoginHandler(c *gin.Context) {
	// TODO: vérifier identifiants, renvoyer JWT
	c.JSON(http.StatusOK, gin.H{"message": "login ok"})
}

func GetPostsHandler(c *gin.Context) {
	// TODO: récupérer les posts depuis la base
	c.JSON(http.StatusOK, gin.H{"posts": []string{"post 1", "post 2"}})
}

func CreatePostHandler(c *gin.Context) {
	// TODO: ajouter post à la base
	c.JSON(http.StatusCreated, gin.H{"message": "post created"})
}
```

Ensuite, dans `cmd/main.go` :
```go
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/QuentinRegnier/nubo-backend/internal/api"
)

func main() {
	r := gin.Default()
	api.SetupRoutes(r)
	r.Run(":8080")
}
```

Tu peux tester les endpoints avec curl ou Postman :
```bash
curl -X POST http://localhost:8080/signup
```
→ tu dois voir :
```json
{"message":"signup ok"}
```

---

## IV. 🚀 Étape WebSocket : créer un serveur WebSocket basique en Go avec Gin + Gorilla WebSocket
### 1. Créer un handler WebSocket

Fichier : `internal/websocket/handler.go`
```go
package websocket

import (
	"net/http"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"log"
)

// Configure le Upgrader (permet de passer du HTTP au WS)
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// TODO : en prod, tu peux vérifier l'origine ici
		return true
	},
}

func WSHandler(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Erreur d'upgrade websocket:", err)
		return
	}
	defer conn.Close()

	log.Println("Client connecté via WebSocket")

	for {
		// Lecture message du client
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("Erreur lecture message:", err)
			break
		}

		log.Printf("Message reçu: %s\n", msg)

		// Envoi d’un message de retour (écho)
		err = conn.WriteMessage(websocket.TextMessage, []byte("Echo: "+string(msg)))
		if err != nil {
			log.Println("Erreur écriture message:", err)
			break
		}
	}
}
```

---

### 2. Brancher le WebSocket dans Gin

Dans `internal/api/routes.go`, ajoute :
```go
package api

import (
	"github.com/gin-gonic/gin"
	"github.com/QuentinRegnier/nubo-backend/internal/websocket"
)

func SetupRoutes(r *gin.Engine) {
	// Routes REST...
	r.POST("/signup", SignUpHandler)
	r.POST("/login", LoginHandler)
	r.GET("/posts", GetPostsHandler)
	r.POST("/posts", CreatePostHandler)

	// WebSocket
	r.GET("/ws", websocket.WSHandler)
}
```

---

### 3. Tester ton WebSocket localement
```bash
websocat ws://localhost:8080/ws
```
Tape un message, tu dois recevoir un `Echo: ton_message`.

---

### 4. Étapes suivantes recommandées après ce test

- Ajouter un gestionnaire de connexions multiples (pool clients broadcast)
- Intégrer Redis Pub/Sub pour scaler (via un canal central)
- Connecter le WebSocket à la base (exemple : notifications)
- Protéger le WS avec JWT (authentification)
- Gérer la reconnexion automatique côté client

---

### 5. Gestionnaire de connexions multiples + broadcast
### Objectif

Garder en mémoire tous les clients connectés, pouvoir diffuser un message à tous en même temps (broadcast).

### 6. Code à ajouter `internal/websocket/hub.go`
```go
package websocket

import (
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type Client struct {
	conn *websocket.Conn
	send chan []byte
}

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.Mutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Println("Client registered")

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				log.Println("Client unregistered")
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

func (c *Client) ReadPump(hub *Hub) {
	defer func() {
		hub.unregister <- c
		c.conn.Close()
	}()
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break
		}
		// On envoie le message reçu à tout le monde
		hub.broadcast <- msg
	}
}

func (c *Client) WritePump() {
	for msg := range c.send {
		err := c.conn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			log.Println("Write error:", err)
			break
		}
	}
	c.conn.Close()
}
```

### 7. Modifier le handler WebSocket `internal/websocket/handler.go`
```go
package websocket

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var hub = NewHub()

func InitHub() {
	go hub.Run()
}

func WSHandler(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	client := &Client{
		conn: conn,
		send: make(chan []byte, 256),
	}

	hub.register <- client

	go client.WritePump()
	client.ReadPump(hub)
}
```

### Initialiser le hub dans `cmd/main.go`
```go
package main

import (
	"github.com/gin-gonic/gin"
	"nubo-backend/internal/api"
	"nubo-backend/internal/websocket"
)

func main() {
	r := gin.Default()

	websocket.InitHub()

	api.SetupRoutes(r)

	r.Run(":8080")
}
```

### 8. Test :
- Lance l’API
- Connecte plusieurs clients au /ws
- Envoie un message d’un client → Tous les clients reçoivent le message

---

## V. Intégration Redis Pub/Sub

Pourquoi ?

Pour permettre à plusieurs instances de ton backend (scalées horizontalement) de communiquer, diffuser les messages WS entre elles.

### 1. Ajouter Redis Pub/Sub dans le hub

Installer Redis client déjà fait avec `go-redis/redis/v8`

### 2. Ajouter un fichier `internal/cache/redis.go`
```go
package cache

import (
	"context"
	"os"

	"github.com/go-redis/redis/v8"
)

var Ctx = context.Background()
var Rdb *redis.Client

func InitRedis() {
	Rdb = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"), // exemple : "localhost:6379"
		Password: "",                      // pas de mot de passe par défaut
		DB:       0,
	})
}
```

### 3. Modifie `cmd/main.go` pour initialiser Redis
```go
package main

import (
	"github.com/gin-gonic/gin"
	"nubo-backend/internal/api"
	"nubo-backend/internal/cache"
	"nubo-backend/internal/websocket"
	"log"
	"os"
)

func main() {
	// Initialiser Redis
	cache.InitRedis()

	// Initialiser Hub WS (modifié pour Redis)
	websocket.InitHub()

	r := gin.Default()
	api.SetupRoutes(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server listening on %s", port)
	r.Run(":" + port)
}
```

### 4. Modifier le hub pour utiliser Redis Pub/Sub

Dans `internal/websocket/hub.go` :

Ajoute en haut :
```go
import (
    "context"
    "log"
    "sync"

    "github.com/go-redis/redis/v8"
    "github.com/gorilla/websocket"
    "github.com/QuentinRegnier/nubo-backend/internal/cache"
)
```
Ajoute dans Hub :
```go
pubsub *redis.PubSub
channel string
Dans NewHub() :

h := &Hub{
	clients:    make(map[*Client]bool),
	broadcast:  make(chan []byte),
	register:   make(chan *Client),
	unregister: make(chan *Client),
	channel:    "nubo-websocket",
}
h.pubsub = cache.Rdb.Subscribe(context.Background(), h.channel)
go h.listenPubSub()
return h
```
Ajoute cette méthode :
```go
func (h *Hub) listenPubSub() {
	ch := h.pubsub.Channel()
	for msg := range ch {
		h.mu.Lock()
		for client := range h.clients {
			select {
			case client.send <- []byte(msg.Payload):
			default:
				close(client.send)
				delete(h.clients, client)
			}
		}
		h.mu.Unlock()
	}
}
```
Modifie la boucle case message := <-h.broadcast dans Run() :
```go
case message := <-h.broadcast:
	// Publie sur Redis
	err := cache.Rdb.Publish(context.Background(), h.channel, message).Err()
	if err != nil {
		log.Println("Redis publish error:", err)
	}
```

---

## VI. Connecter le WebSocket à la base (exemple notifications)

### 1. Exemple rapide

Dans Client.ReadPump(), à chaque message reçu, tu peux enregistrer ou traiter en DB.

### 2. Exemple dans `internal/websocket/hub.go` ReadPump :
```go
func (c *Client) ReadPump(hub *Hub) {
	defer func() {
		hub.unregister <- c
		c.conn.Close()
	}()
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break
		}

		// TODO: sauvegarder msg en base (Postgres/Mongo)

		// Envoie le message aux autres clients
		hub.broadcast <- msg
	}
}
```

---

## VII. Protéger le WS avec JWT

### 1. Ajouter middleware d’authentification JWT
Télécharger avant :
```bash
go get github.com/golang-jwt/jwt/v5
```

Dans `internal/api/middleware.go` :
```go
package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte("ta-cle-secrete") // à sécuriser via .env

func JWTMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header manquant"})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token invalide"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Claims invalides"})
			return
		}

		// Stocke l’ID utilisateur dans le contexte
		c.Set("userID", claims["sub"])

		c.Next()
	}
}
```

### 2. Protéger la route WebSocket

Dans `internal/api/routes.go` :
```go
r.GET("/ws", JWTMiddleware(), websocket.WSHandler)
```
Dans `internal/websocket/handler.go`, récupérer l’ID utilisateur (exemple)
```go
func WSHandler(c *gin.Context) {
	userID := c.GetString("userID")
	log.Println("Utilisateur connecté (userID):", userID)

	// … reste inchangé
}
```

---

### 3. ✅ PRÉREQUIS AVANT DE TESTER

- Redis tourne bien (docker-compose up)
- Ton backend Go est lancé (go run cmd/main.go)
Tu as un JWT valide (car le WS est protégé maintenant) — on en génère un dans l'étape 1 👇

---

### 4. 🧪 Générer un JWT de test

**Ajoute un petit endpoint temporaire dans internal/api/routes.go juste pour tester :**
```go
import "github.com/golang-jwt/jwt/v5"

func SetupRoutes(r *gin.Engine) {
	r.GET("/token", func(c *gin.Context) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": "user123", // ID utilisateur
		})
		tokenString, _ := token.SignedString([]byte("ta-cle-secrete"))
		c.JSON(200, gin.H{"token": tokenString})
	})

	r.GET("/ws", JWTMiddleware(), websocket.WSHandler)
}
```

Lance ton API puis exécute :
```bash
curl http://localhost:8080/token
```
Tu obtiens une réponse comme :
```json
{"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6Ikp..."}
```

---

### 5. 🧪 Se connecter avec websocat + JWT

**Copie le token et lance cette commande dans le terminal :**
```bash
websocat ws://localhost:8081/ws -H 'Authorization: Bearer Remplace TON_JWT_ICI'
```
Remplace TON_JWT_ICI par le vrai token.

**✅ Tu dois voir dans les logs :**
```bash
Utilisateur connecté (userID): user123
Client registered
```

---

### 6. 🧪 Tester le broadcast

- Ouvre deux terminaux avec cette même commande websocat (et le même token).
- Tape un message dans l’un. Tu dois le recevoir dans les deux.

**💬 Exemple :**
```bash
> Salut à tous !
```

🟢 Les deux clients doivent afficher Salut à tous !.

---

### 7. 🧪 Tester le Redis Pub/Sub (scalabilité)

**Étapes :**
- Lance une deuxième instance de ton backend (dans un autre terminal) :
```bash
go run cmd/main.go
```
- Connecte websocat à chaque instance :
Instance 1 : port `8080`
Instance 2 : change à la volée : `r.Run(":8081")`, connecte à `ws://localhost:8081/ws`
- Envoie un message sur une instance, il doit apparaître sur toutes → preuve que Redis propulse les messages entre les processus backend.

---

### 8. 🧪 Vérifie le JWT en cas d'erreur

Si tu connectes sans Authorization, tu dois recevoir :
```bash
{"error":"Authorization header manquant"}
```
Ou :
```bash
{"error":"Token invalide"}
```
## VIII. PostgreSQL & MongoDB
### 1. Mise en place de PostgreSQL avec pgAdmin
**Créer la structure Docker pour PostgreSQL + pgAdmin**
Dans `docker-compose.yml`, ajoute la section PostgreSQL + pgAdmin :
```yaml
version: '3.8'

services:
  postgres:
    image: postgres:16
    container_name: nubo_postgres
    environment:
      POSTGRES_USER: nubo_user
      POSTGRES_PASSWORD: nubo_password
      POSTGRES_DB: nubo_db
    ports:
      - "5432:5432"
    volumes:
      - ./postgres-data:/var/lib/postgresql/data

  pgadmin:
    image: dpage/pgadmin4
    container_name: nubo_pgadmin
    environment:
      PGADMIN_DEFAULT_EMAIL: admin@nubo.com
      PGADMIN_DEFAULT_PASSWORD: admin
    ports:
      - "8082:80"
```
**Démarre les services :**
`docker-compose -f docker-compose.yml up -d`
**Résultat attendu :**
PostgreSQL accessible sur `localhost:5432`
pgAdmin accessible sur http://localhost:8082
Tu peux connecter pgAdmin à PostgreSQL avec nubo_user / nubo_password
**Créer les dossiers pour les scripts SQL**
Arborescence suggérée :
```bash
Nubo/
├─ docker/
│  └─ docker-compose.yml
├─ sql/
│  ├─ init/       # Scripts de création de tables de base
│  ├─ functions/  # Procédures stockées, triggers
│  └─ views/      # Vues SQL
```
Place tes fichiers `.sql` dans ces dossiers selon leur rôle.

---

### 2. Mise en place de MongoDB avec mongo-express
**Ajouter MongoDB à Docker**
Toujours dans `docker-compose.yml`, ajoute :
```yaml
  mongo:
    image: mongo:7
    container_name: nubo_mongo
    ports:
      - "27017:27017"
    volumes:
      - ./mongo-data:/data/db

  mongo-express:
    image: mongo-express:1.0.0
    container_name: nubo_mongo_express
    environment:
      ME_CONFIG_MONGODB_ADMINUSERNAME: root
      ME_CONFIG_MONGODB_ADMINPASSWORD: example
      ME_CONFIG_MONGODB_SERVER: mongo
    ports:
      - "8083:8081"
```
**Démarre le service :**
`docker-compose -f docker-compose.yml up -d`
MongoDB accessible sur `localhost:27017`
mongo-express accessible sur http://localhost:8083
**Arborescence pour les scripts Mongo**
```bash
Nubo/
├─ mongo/
│  ├─ init/          # Création des collections, index, sharding
│  ├─ scripts/       # Inserts, indexes, TTL commands
│  └─ workers/       # Workers pour traitement asynchrone
```

---

## IX. Écriture dans les bases
### 1. Règle d’or (rappel rapide)
- **PostgreSQL** = système de vérité, intégrité, contraintes, requêtes relationnelles (users, follow, paramètres, archival à vie).
- **MongoDB** = charge volatile/haut-volume, accès par documents, données massives / flexibles / dernière période (cache, messages récents, logs).
- Redis = stockage en mémoire pour sessions / liste d’utilisateurs en ligne / counters / pubsub (déjà en place).

### 2. Arborescence globale (haute niveau)
```bash
PostgreSQL
├─ users
│  ├─ user_settings
│  ├─ sessions (refresh tokens / audit)
├─ follows
├─ blocks
├─ posts
│  ├─ comments
│  ├─ likes
├─ media (metadata minimal + pointer vers stockage objet)
├─ conversations_meta
│  ├─ conversation_members
│  ├─ message_index (résumé/offset pour messages dans Mongo)
├─ reports
├─ admin_actions
└─ audit_logs

MongoDB
├─ messages                      (messages complets / growth)
├─ posts_documents               (post + media metadata volumineux / versions)
├─ comments_documents            (ou embedded in posts if small)
├─ notifications                 (push & in-app notifications)
├─ feed_cache                    (pré-calculé, TTL)
├─ user_activity_logs            (clicks, views, events)
├─ media_metadata                (vision/thumbnail/ai-tags)
└─ search_index / embeddings     (opti. pour recommandations)
```

### 3. PostgreSQL : Tables clefs (schéma résumé)

> Utiliser UUID (uuid_generate_v4()) pour tous les id en production. Datetime en timestamptz.

**PostgreSQL :**

**users**
- `id uuid PRIMARY KEY`
- `username text UNIQUE NOT NULL`
- `email text UNIQUE NOT NULL`
- `password_hash text NOT NULL`
- `salt text NULL` (si tu utilises scrypt/bcrypt, pas nécessaire)
- `display_name text`
- `bio text`
- `birthdate date`
- `phone text UNIQUE NULL`
- `profile_picture_id uuid NULL` (référence dans media)
- `state smallint NOT NULL DEFAULT 1` (1=active, 0=deleted, 2=banned)
- `created_at timestamptz DEFAULT now()`
- `updated_at timestamptz`
**Index** : `ON users (username)`, `ON users (email)`.
```sql
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique de l'utilisateur
    username TEXT UNIQUE NOT NULL, -- nom d'utilisateur unique
    email TEXT UNIQUE NOT NULL, -- email unique
    email_verified BOOLEAN DEFAULT FALSE, -- email vérifié
    phone TEXT UNIQUE, -- numéro de téléphone unique
    phone_verified BOOLEAN DEFAULT FALSE, -- numéro de téléphone vérifié
    password_hash TEXT NOT NULL, -- mot de passe haché
    first_name TEXT NOT NULL, -- prénom
    last_name TEXT NOT NULL, -- nom de famille
    birthdate DATE, -- date de naissance
    sex SMALLINT, -- sexe
    bio TEXT, -- biographie
    profile_picture_id UUID, -- id de l'image de profil
    grade SMALLINT NOT NULL DEFAULT 1, -- grade de l'utilisateur
    location TEXT, -- localisation de l'utilisateur
    school TEXT, -- école
    works TEXT, -- emplois
    badges TEXT[], -- badges
    created_at TIMESTAMPTZ DEFAULT now(), -- date de création
    updated_at TIMESTAMPTZ DEFAULT now() -- date de mise à jour
);

CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_email ON users(email);
```

---

**user_settings**
- `id uuid PRIMARY KEY`
- `user_id uuid REFERENCES users(id) UNIQUE ON DELETE CASCADE`
- `privacy jsonb` (ex: {"posts":"public","messages":"friends"})
- `notifications jsonb` (per-type on/off)
- `language text`
- `theme text`
**Index** : `ON user_settings (user_id)`.
```sql
CREATE TABLE IF NOT EXISTS user_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique des paramètres utilisateur
    user_id UUID UNIQUE REFERENCES users(id) ON DELETE CASCADE, -- id unique de l'utilisateur
    privacy JSONB, -- paramètres de confidentialité
    notifications JSONB, -- paramètres de notification
    language TEXT, -- langue
    theme SMALLINT NOT NULL DEFAULT 0 -- thème clair/sombre
);

CREATE INDEX idx_user_settings_user_id ON user_settings(user_id);
```
---

**sessions (refresh tokens / audit)**
- `id uuid PRIMARY KEY`
- `user_id uuid REFERENCES users(id)`
- `refresh_token text`
- `device_info jsonb`
- `ip inet`
- `created_at timestamptz`
- `expires_at timestamptz`
- `revoked boolean DEFAULT false`
**Index** : `ON sessions (user_id, revoked)`.
```sql
CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique de la session
    user_id UUID REFERENCES users(id), -- id de l'utilisateur
    refresh_token TEXT, -- token de rafraîchissement
    device_info JSONB, -- informations sur l'/les appareil(s)
    ip INET[], -- adresse IP
    created_at TIMESTAMPTZ DEFAULT now(), -- date de création
    expires_at TIMESTAMPTZ, -- date d'expiration
    revoked BOOLEAN DEFAULT FALSE -- session révoquée
);

CREATE INDEX idx_sessions_user_id_revoked ON sessions(user_id, revoked);
```
---

**follows**
- `id uuid PRIMARY KEY`
- `follower_id uuid REFERENCES users(id)`
- `followed_id uuid REFERENCES users(id)`
- `state smallint DEFAULT 1 (1=ok, 0=pending, 2=blocked)`
- `created_at timestamptz`
**Unique constraint** : `(follower_id, followed_id)`.
**Index** : `ON follows (followed_id)` pour feed queries.
```sql
CREATE TABLE IF NOT EXISTS follows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique du suivi
    followed_id UUID REFERENCES users(id), -- id de l'utilisateur suivi
    state SMALLINT DEFAULT 1, -- état du suivi (2 = amis, 1 = suivi, 0 = inactif, -1 = bloqué)
    created_at TIMESTAMPTZ DEFAULT now(), -- date de création
    UNIQUE(follower_id, followed_id)
);

CREATE INDEX idx_follows_followed_id ON follows(followed_id);
```
---

**posts**
- `id uuid PRIMARY KEY`
- `user_id uuid REFERENCES users(id) NOT NULL`
- `content text` (short textual content)
- `media_ids uuid[]` (pointeurs vers table media ou Mongo)
- `meta jsonb` (mentions, hashtags, extra props)
- `visibility smallint` (0=private,1=friends,2=public)
- `created_at timestamptz`
- `updated_at timestamptz`
**Index** : `ON posts (user_id, created_at DESC)` ; `GIN index ON posts (meta jsonb)` pour recherches.
```sql
CREATE TABLE IF NOT EXISTS posts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique du post
    user_id UUID REFERENCES users(id) NOT NULL, -- id de l'utilisateur
    content TEXT, -- contenu du post
    media_ids UUID[], -- ids des médias associés
    meta JSONB, -- métadonnées
    visibility SMALLINT DEFAULT 0, -- visibilité (1 = amis, 0 = public)
    location TEXT, -- localisation
    created_at TIMESTAMPTZ DEFAULT now(), -- date de création
    updated_at TIMESTAMPTZ DEFAULT now() -- date de mise à jour
);

CREATE INDEX idx_posts_user_created ON posts(user_id, created_at DESC);
CREATE INDEX idx_posts_meta ON posts USING GIN(meta);
```
---

**comments**
- `id uuid PRIMARY KEY`
- `post_id uuid REFERENCES posts(id) ON DELETE CASCADE`
- `user_id uuid REFERENCES users(id)`
- `content text`
- `created_at timestamptz`
**Index:** `ON comments (post_id, created_at DESC)`.
```sql
CREATE TABLE IF NOT EXISTS comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique du commentaire
    post_id UUID REFERENCES posts(id) ON DELETE CASCADE, -- id du post
    user_id UUID REFERENCES users(id), -- id de l'utilisateur
    content TEXT, -- contenu du commentaire
    created_at TIMESTAMPTZ DEFAULT now() -- date de création
);

CREATE INDEX idx_comments_post_created ON comments(post_id, created_at DESC);
```
---

**likes**
- `id uuid PRIMARY KEY`
- `target_type text (post/comment)`
- `target_id uuid`
- `user_id uuid REFERENCES users(id)`
- `created_at timestamptz`
**Unique** `(target_type, target_id, user_id)`.
**Index** : `ON likes (target_type, target_id)`.
```sql
CREATE TABLE IF NOT EXISTS likes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique du like
    target_type SMALLINT NOT NULL, -- type de la cible (0 = post, 1 = message, 2 = commentaire)
    target_id UUID NOT NULL, -- id de la cible
    user_id UUID REFERENCES users(id), -- id de l'utilisateur
    created_at TIMESTAMPTZ DEFAULT now(), -- date de création
    UNIQUE(target_type, target_id, user_id)
);

CREATE INDEX idx_likes_target ON likes(target_type, target_id);
```
---

**conversations_meta**
- `id uuid PRIMARY KEY`
- `type smallint` (1=direct,2=group)
- `title text NULL`
- `last_message_text text NULL`
- `last_message_time timestamptz NULL`
- `created_at timestamptz`
**Index** : `ON conversations_meta (last_message_time DESC)`.
```sql
CREATE TABLE IF NOT EXISTS conversations_meta (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique de la conversation
    type SMALLINT, -- type de la conversation (0 = message privée, 1 = groupe, 2 = communauté, 3 = annonce)
    title TEXT, -- titre de la conversation
    last_message_id UUID UNIQUE, -- id du dernier message
    state SMALLINT DEFAULT 0, -- état de la conversation (0 = active, 1 = archivée, 2 = supprimée)
    created_at TIMESTAMPTZ DEFAULT now() -- date de création
);

CREATE INDEX idx_conversations_last_message ON conversations_meta(last_message_id);
```
---

**conversation_members**
- `id uuid PRIMARY KEY`
- `conversation_id uuid REFERENCES conversations_meta(id)`
- `user_id uuid REFERENCES users(id)`
- `role smallint` (admin/member)
- `joined_at timestamptz`
- `unread_count int DEFAULT 0`

**Unique** `(conversation_id, user_id)`.
```sql
CREATE TABLE IF NOT EXISTS conversation_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique du membre
    conversation_id UUID REFERENCES conversations_meta(id), -- id de la conversation
    user_id UUID REFERENCES users(id), -- id de l'utilisateur
    role SMALLINT DEFAULT 0, -- rôle du membre (0 = membre, 1 = admin, 2 = créateur)
    joined_at TIMESTAMPTZ DEFAULT now(), -- date d'adhésion
    unread_count INT DEFAULT 0, -- nombre de messages non lus
    UNIQUE(conversation_id, user_id)
);
```
---

**message_index (résumé pour accès rapide)**
- `id uuid PRIMARY KEY` (same as Mongo message id or index)
- `conversation_id uuid REFERENCES conversations_meta(id)`
- `message_id text` (id in Mongo or pointer)
- `sender_id uuid`
- `created_at timestamptz`
- `snippet text` (first X chars)
**Index**: `ON message_index (conversation_id, created_at DESC)`.
**Pattern** : on écrit le message complet dans MongoDB (champ texte, medias), et on écrit une ligne d’index/minimale en Postgres (message_index) pour permettre recherche pagination rapide, jointures, quotas, unread counters, etc.
```sql
CREATE TABLE IF NOT EXISTS message_index (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique du message
    conversation_id UUID REFERENCES conversations_meta(id), -- id de la conversation
    sender_id UUID NOT NULL, -- id de l'expéditeur
    message_type SMALLINT NOT NULL DEFAULT 0, -- 0=text, 1=image, 2=publication, 3=vocal, 4=vidéo
    content TEXT, -- contenu du message
    attachments JSONB, -- pointeurs vers fichiers S3 / metadata
    created_at TIMESTAMPTZ DEFAULT now(), -- date de création
);


CREATE INDEX idx_message_index_conv_created ON message_index(conversation_id, created_at DESC);
```
---

**media (minimum metadata)**
- `id uuid PRIMARY KEY`
- `owner_id uuid`
- `storage_path text` (S3 path)
- `mime text`
- `size bigint`
- `width int, height int`
- `created_at timestamptz`
- `processing_state smallint` (0=pending,1=done)
**Index** : `ON media (owner_id)`.
```sql
CREATE TABLE IF NOT EXISTS media (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique du média
    owner_id UUID REFERENCES users(id), -- id du propriétaire
    storage_path TEXT, -- chemin de stockage
    created_at TIMESTAMPTZ DEFAULT now(), -- date de création
);

CREATE INDEX idx_media_owner ON media(owner_id);
CREATE INDEX idx_media_created ON media(created_at);
```
---

**reports**
```sql
CREATE TABLE IF NOT EXISTS reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique du rapport
    actor_id UUID REFERENCES users(id), -- id de l'utilisateur ayant signalé
    target_type SMALLINT NOT NULL, -- type de la cible (user/post/comment/etc)
    target_id UUID NOT NULL, -- id de la cible
    reason TEXT, -- raison du signalement
    state SMALLINT DEFAULT 0, -- état du rapport (0=pending, 1=reviewed, 2=resolved)
    created_at TIMESTAMPTZ DEFAULT now() -- date de création
);

CREATE INDEX idx_reports_actor ON reports(actor_id);
CREATE INDEX idx_reports_created ON reports(created_at);
```

### 4. PosgreSQL : Schéma
```bash
📂 database/
 ├── 📂 schemas/
 │    ├── auth/          → utilisateurs, sessions, relations
 │    ├── content/       → posts, comments, likes, media
 │    ├── messaging/     → conversations, messages
 │    ├── moderation/    → reports
 │    ├── logic/         → fonctions + procédures
 │    ├── views/         → vues matérialisées ou non
```

---

### 5. MongoDB
> Principe : stocker documents volumineux, formats libres, TTL sur ce qui est éphémère.
**MongoExpress**
- Créer une nouvelle base `nubo_recent`
**Terminal**
1. Mets ton Homebrew à jour :
```bash
brew update
```
2. Installe mongosh :
```bash
brew install mongosh
```
3. Vérifie que ça marche :
```bash
mongosh --version
```
**Se connecter à ton serveur Mongo**
Une fois installé, tu pourras te connecter à ton serveur MongoDB (celui où Mongo Express est branché).
En général, si c’est en local :
```bash
mongosh "mongodb://root:example@localhost:27017"
```
Tu choisis la base (si ce n’est pas encore fait) :
```javascript
use nubo_recent
```
Ensuite tu colles ton script complet :
(voir collection)
---

**messages (collection)**
```javascript
// ---------------------- USERS ----------------------
db.createCollection("users_recent");
db.users_recent.createIndex({ username: 1 }, { unique: true });
db.users_recent.createIndex({ email: 1 }, { unique: true });

// Exemple d’insertion complète pour users_recent
db.users_recent.insertOne({
    _id: UUID(),
    username: "",
    email: "",
    email_verified: false,
    phone: null,
    phone_verified: false,
    password_hash: "",
    first_name: "",
    last_name: "",
    birthdate: null,
    sex: null,
    bio: "",
    profile_picture_id: null,
    grade: 1,
    location: "",
    school: "",
    work: "",
    badges: [],
    created_at: new Date(),
    updated_at: new Date(),
    connected: false
});

// ---------------------- USER SETTINGS ----------------------
db.createCollection("user_settings_recent");
db.user_settings_recent.createIndex({ user_id: 1 }, { unique: true });

db.user_settings_recent.insertOne({
    _id: UUID(),
    user_id: UUID(),
    privacy: {},
    notifications: {},
    language: "",
    theme: 0,
    created_at: new Date(),
    updated_at: new Date()
});

// ---------------------- SESSIONS ----------------------
db.createCollection("sessions_recent");
db.sessions_recent.createIndex({ user_id: 1, revoked: 1 });

db.sessions_recent.insertOne({
    _id: UUID(),
    user_id: UUID(),
    refresh_token: "",
    device_info: {},
    ip: [],
    created_at: new Date(),
    expires_at: null,
    revoked: false
});

// ---------------------- RELATIONS ----------------------
db.createCollection("relations_recent");
db.relations_recent.createIndex({ primary_id: 1 });
db.relations_recent.createIndex({ secondary_id: 1 });
db.relations_recent.createIndex({ secondary_id: 1, primary_id: 1 }, { unique: true });

db.relations_recent.insertOne({
    _id: UUID(),
    primary_id: UUID(),
    secondary_id: UUID(),
    state: 1,
    created_at: new Date()
});

// ---------------------- POSTS ----------------------
db.createCollection("posts_recent");
db.posts_recent.createIndex({ user_id: 1, created_at: -1 });

db.posts_recent.insertOne({
    _id: UUID(),
    user_id: UUID(),
    content: "",
    media_ids: [],
    visibility: 0,
    location: "",
    created_at: new Date(),
    updated_at: new Date()
});

// ---------------------- COMMENTS ----------------------
db.createCollection("comments_recent");
db.comments_recent.createIndex({ post_id: 1, created_at: -1 });

db.comments_recent.insertOne({
    _id: UUID(),
    post_id: UUID(),
    user_id: UUID(),
    content: "",
    created_at: new Date()
});

// ---------------------- LIKES ----------------------
db.createCollection("likes_recent");
db.likes_recent.createIndex({ target_type: 1, target_id: 1 });
db.likes_recent.createIndex({ target_type: 1, target_id: 1, user_id: 1 }, { unique: true });

db.likes_recent.insertOne({
    _id: UUID(),
    target_type: 0,
    target_id: UUID(),
    user_id: UUID(),
    created_at: new Date()
});

// ---------------------- MEDIA ----------------------
db.createCollection("media_recent");
db.media_recent.createIndex({ owner_id: 1 });
db.media_recent.createIndex({ created_at: 1 });

db.media_recent.insertOne({
    _id: UUID(),
    owner_id: UUID(),
    storage_path: "",
    created_at: new Date()
});

// ---------------------- CONVERSATIONS META ----------------------
db.createCollection("conversations_recent");
db.conversations_recent.createIndex({ last_message_id: 1 });

db.conversations_recent.insertOne({
    _id: UUID(),
    type: 0,
    title: "",
    last_message_id: null,
    state: 0,
    created_at: new Date()
});

// ---------------------- CONVERSATION MEMBERS ----------------------
db.createCollection("conversation_members_recent");
db.conversation_members_recent.createIndex({ conversation_id: 1, user_id: 1 }, { unique: true });

db.conversation_members_recent.insertOne({
    _id: UUID(),
    conversation_id: UUID(),
    user_id: UUID(),
    role: 0,
    joined_at: new Date(),
    unread_count: 0
});

// ---------------------- MESSAGES ----------------------
db.createCollection("messages_recent");
db.messages_recent.createIndex({ conversation_id: 1, created_at: -1 });

db.messages_recent.insertOne({
    _id: UUID(),
    conversation_id: UUID(),
    sender_id: UUID(),
    message_type: 0,
    state: 0,
    content: "",
    attachments: {},
    created_at: new Date()
});

// ---------------------- FEED CACHE ----------------------
db.createCollection("feed_cache");
db.feed_cache.createIndex({ user_id: 1, created_at: -1 });

db.feed_cache.insertOne({
    _id: UUID(),
    user_id: UUID(),
    items: [],
    created_at: new Date()
});
```

---

## X. Stratégie de requêtes des données :

### 1. Stratégie MongoDB réajustée
1. Répliquer uniquement les données “interactives” du dernier mois :
- Interactions = lecture, écriture, modification, likes, commentaires, etc.
- MongoDB ne reçoit que ce sous-ensemble des tables concernées (`users`, `sessions`, `posts`, `comments`, `likes`, `media`, `messages`, `conversations_meta`, `conversation_members`, `relations`).
- On ne fait pas de réplication totale. C’est donc bien un filtrage côté Go, pas PostgreSQL.
2. Feed pré-calculé
- Continu pour les utilisateurs connectés.
- Occasionnel pour les utilisateurs non connectés, selon la charge serveur.
3. Décision de répliquer / stocker les données :
- Exclusivement côté Go, qui connaît la logique métier et peut filtrer les données “récentes ou actives”.
- PostgreSQL n’est utilisé que comme source de vérité pour les données anciennes ou massives.
4. Lecture multi-couche :
```text
Go cherche un message/post :
-> Redis (cache ultra rapide)
-> Mongo (données récentes ou lourdes)
-> PostgreSQL (historique ou requêtes complexes)
```
- On peut sauter des étapes si on sait déjà que la donnée est ancienne ou que le filtre limite à moins d’un mois.
5. Écriture / suppression :
- Écriture triple : Redis + Mongo + PostgreSQL.
- Suppression / update : idem, pour garder la cohérence.

---

### 2. Optimisation de la fil d’attente et des écritures massives
**PostgreSQL**
- Batch insertions : plutôt que d’écrire 50 000 lignes une par une, grouper les inserts dans une seule requête `INSERT ... VALUES (...), (...), (...)`.
- Transactions groupées : encapsuler plusieurs opérations dans une seule transaction réduit les commits, ce qui accélère les écritures et limite la fragmentation.
- COPY : pour des gros volumes, `COPY FROM` est beaucoup plus rapide qu’un `INSERT` classique.
- Prepared statements : si on fait beaucoup d’inserts similaires, préparer la requête et l’exécuter en boucle réduit l’overhead.
- Indexes : désactiver temporairement certains indexes pendant un bulk insert massif puis les reconstruire peut être plus rapide.
**MongoDB**
- insertMany : Mongo gère très bien les insertions en masse via `insertMany`.
- Ordered=false : permet de continuer l’insertion même si certains documents échouent, utile pour les très gros batchs.
- Bulk API : `bulkWrite` permet de combiner insert, update, delete dans une seule opération, très efficace pour la réplication / traitement de flux.
- Sharding : si le dataset devient massif, sharder sur une clé qui répartit uniformément la charge d’écriture (ex : `conversation_id` pour messages).
- Write concern : ajuster le write concern (`w=1` pour rapide, `w=majority` pour sûr) selon le besoin.
**Général**
- Parallelisation côté Go :
	- Regrouper les écritures par type et table.
	- Faire plusieurs goroutines pour envoyer les batchs en parallèle.
	- Redis est naturellement rapide pour des mises à jour concurrentes.

---

### 3. Schéma conceptuel clair du flux multi-couche
```pgsql
                        ┌───────────────────────┐
                        │       Utilisateur      │
                        │   (Mobile / Web)      │
                        └───────────┬───────────┘
                                    │
                                    ▼
                           ┌─────────────────┐
                           │       Go        │
                           │  Orchestrateur  │
                           │   logique métier│
                           └───────┬─────────┘
                                   │
      ┌────────────────────────────┼────────────────────────────┐
      │                            │                            │
      ▼                            ▼                            ▼
┌───────────────┐           ┌─────────────────┐         ┌─────────────────┐
│     Redis     │           │     MongoDB     │         │  PostgreSQL     │
│  Cache rapide │           │ Données récentes│         │ Source de vérité│
│  - unread     │           │ < 1 mois /      │         │ historique      │
│    counters   │           │ interactions    │         │ - toutes tables │
│  - sessions   │           │ - messages      │         │ - contraintes   │
│  - pub/sub    │           │ - posts volum.  │         │   d’intégrité   │
│  - feed cache │           │ - conversations │         │ - requêtes      │
└───────────────┘           │   récentes      │         │   complexes     │
                            │ - medias récents│         └─────────────────┘
                            └───────┬─────────┘
                                    │
                       ┌────────────┴──────────────┐
                       │   Batch / Bulk insertions │
                       │   insertMany / COPY       │
                       │   Parallelisation Go      │
                       └───────────────────────────┘
```

---

**Explications du flux**
1. Utilisateur interagit → envoie une requête à Go.
2. Go décide :
	- Lire → Redis → MongoDB → PostgreSQL si nécessaire.
	- Écrire → Redis + MongoDB + PostgreSQL.
	- Supprimer → Redis + MongoDB + PostgreSQL.
3. MongoDB contient uniquement les données récentes ou utilisées activement (moins d’un mois, interactions récentes).
4. Redis sert pour :
	- compteur de messages non lus,
	- sessions actives,
	- pub/sub temps réel,
	- feed cache temporaire.
5. PostgreSQL reste la source de vérité complète, historique, contraintes d’intégrité, et requêtes complexes (rapports, exports, analytics).

---

**Optimisation / charge serveur**
- Go peut batcher les insertions :
	- Messages, posts, commentaires → insertMany pour Mongo, COPY ou multi-row insert pour PostgreSQL.
- Feed pré-calculé :
	- Pour les utilisateurs connectés → continu.
	- Pour les non-connectés → seulement quand charge CPU/RAM le permet (heures creuses).
- Lecture / filtre :
	- Pré-filtrer par moins d’un mois → MongoDB.
	- Si besoin historique → PostgreSQL.

## XI. Travail sur Go
### 1. Initialisation de Redis et Mongo :
__**Objectif :**__
- Forcer à la supprésion toutes les lignes ayant été utilisé il y a plus d'un mois dans les collections de MongoDB dans la base `nubo_recent`
- Nettoyer totalement Redis

**Création de `init.go` et insertion de la directive dans `main.go`**
```go
package initdata

import (
    "context"
    "log"
    "time"

    "github.com/QuentinRegnier/nubo-backend/internal/cache"
    "github.com/QuentinRegnier/nubo-backend/internal/db"
    "go.mongodb.org/mongo-driver/bson"
)

func CleanMongo() {
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()

    dbRecent := db.MongoClient.Database("nubo_recent")

    // Récupère toutes les collections de la DB
    collections, err := dbRecent.ListCollectionNames(ctx, bson.D{})
    if err != nil {
        log.Printf("❌ Erreur récupération collections Mongo: %v", err)
        return
    }

    // Date limite : 30 jours
    threshold := time.Now().AddDate(0, 0, -30)

    for _, collName := range collections {
        coll := dbRecent.Collection(collName)

        // Supprime les documents dont last_use < threshold
        filter := bson.M{
            "last_use": bson.M{
                "$lt": threshold,
            },
        }

        res, err := coll.DeleteMany(ctx, filter)
        if err != nil {
            log.Printf("❌ Erreur suppression dans %s: %v", collName, err)
            continue
        }

        log.Printf("🧹 Nettoyage Mongo [%s] → %d documents supprimés", collName, res.DeletedCount)
    }
}

func CleanRedis() {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    err := cache.Rdb.FlushDB(ctx).Err()
    if err != nil {
        log.Printf("❌ Erreur flush Redis: %v", err)
        return
    }
    log.Println("🧹 Redis vidé avec succès ✅")
}

func InitData() {
    log.Println("=== Initialisation: Nettoyage Mongo + Redis ===")
    CleanMongo()
    CleanRedis()
    log.Println("=== Initialisation terminée ✅ ===")
}
```
```go
// Nettoyage au démarrage
    initdata.InitData()
```

---

### 2. Sécurisation par JWT 
- mise en place d'une expiration dans le token
- mise en place de la connexion du JWT_SCRET dans celui dans `.env`
```go 
package api

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func JWTMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.GetHeader("Authorization")
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header manquant"})
			return
		}

		// Retirer "Bearer " si présent
		if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
			tokenString = tokenString[7:]
		}

		keyFunc := func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("algorithme JWT invalide")
			}
			return []byte(os.Getenv("JWT_SECRET")), nil
		}

		token, err := jwt.Parse(tokenString, keyFunc)
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token invalide"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Claims invalides"})
			return
		}

		// Vérification expiration
		if exp, ok := claims["exp"].(float64); ok {
			if time.Now().After(time.Unix(int64(exp), 0)) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token expiré"})
				return
			}
		}

		// Mettre l’ID utilisateur dans le contexte
		c.Set("userID", claims["sub"])

		c.Next()
	}
}
```

---

### 3. Dynamisation de la gestion de la RAM pour REDIS et création de type de noeud
- type **flux** : permet d'envoyer des données à d'autre WS, durée de vie des données 1s
- type **cache** : permet de stocker des données pour soulager MongoDB et PostgreSQL ainsi que pour augmenter la vitesse, durée de vie des données infini ou presque
- gestion intélligente de la RAM avec une marge laissé vide à ne pas dépasser sinon le programme purge les données les moins utilisé de REDIS, ce sont bien les données qui sont purger et pas les noeuds entier
```go
// internal/cache/strategy_redis.go
package cache

import (
	"context"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// ---------------- Types ----------------

// Type d’un noeud Redis
type NodeType int

const (
	NodeFlux NodeType = iota
	NodeCache
)

// Un élément dans la LRU globale
type CacheElement struct {
	NodeName  string // nom du noeud (ex: "messages")
	ElementID string // ex: "392"
	prev      *CacheElement
	next      *CacheElement
}

// LRU globale pour les éléments de type cache
type LRUCache struct {
	elements map[string]*CacheElement // clé = nodeName:elementID
	head     *CacheElement
	tail     *CacheElement
	mu       sync.Mutex
	rdb      *redis.Client
}

// ---------------- Initialisation ----------------

// NewLRUCache initialise un cache LRU global
func NewLRUCache(rdb *redis.Client) *LRUCache {
	return &LRUCache{
		elements: make(map[string]*CacheElement),
		rdb:      rdb,
	}
}

// ---------------- Gestion usage ----------------

// MarkUsed marque un élément comme utilisé (move to tail)
func (lru *LRUCache) MarkUsed(nodeName, elementID string) {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	key := nodeName + ":" + elementID
	elem, exists := lru.elements[key]
	if exists {
		lru.moveToTail(elem)
		return
	}

	elem = &CacheElement{NodeName: nodeName, ElementID: elementID}
	lru.elements[key] = elem
	lru.append(elem)
}

func (lru *LRUCache) moveToTail(elem *CacheElement) {
	if elem == lru.tail {
		return
	}
	lru.remove(elem)
	lru.append(elem)
}

func (lru *LRUCache) append(elem *CacheElement) {
	if lru.tail != nil {
		lru.tail.next = elem
		elem.prev = lru.tail
		elem.next = nil
		lru.tail = elem
	} else {
		lru.head = elem
		lru.tail = elem
	}
}

func (lru *LRUCache) remove(elem *CacheElement) {
	if elem.prev != nil {
		elem.prev.next = elem.next
	} else {
		lru.head = elem.next
	}
	if elem.next != nil {
		elem.next.prev = elem.prev
	} else {
		lru.tail = elem.prev
	}
	elem.prev = nil
	elem.next = nil
}

func (lru *LRUCache) purgeOldest() {
	if lru.head == nil {
		return
	}
	old := lru.head
	log.Printf("Purging Redis cache element (LRU): node=%s, id=%s\n", old.NodeName, old.ElementID)
	lru.remove(old)
	delete(lru.elements, old.NodeName+":"+old.ElementID)

	// suppression dans Redis
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	key := "cache:" + old.NodeName
	if err := lru.rdb.HDel(ctx, key, old.ElementID).Err(); err != nil {
		log.Printf("Erreur suppression Redis: %v\n", err)
	}
}

// ---------------- Mémoire ----------------

// StartMemoryWatcher surveille la RAM
func (lru *LRUCache) StartMemoryWatcher(maxRAM uint64, marge uint64, interval time.Duration) {
	go func() {
		for {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			used := m.Alloc
			if maxRAM == 0 {
				maxRAM = getTotalRAM()
			}
			if used > maxRAM-marge {
				log.Printf("RAM utilisée=%d, dépassement seuil=%d, purge LRU...\n", used, maxRAM-marge)
				lru.mu.Lock()
				lru.purgeOldest()
				lru.mu.Unlock()
			}
			time.Sleep(interval)
		}
	}()
}

func getTotalRAM() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Sys
}

// ---------------- Flux ----------------

// DefaultFluxTTL est le temps de vie par défaut d'un message de flux
const DefaultFluxTTL = 1 * time.Second

// PushFluxWithTTL publie un message sur un flux et crée un TTL individuel
func PushFluxWithTTL(rdb *redis.Client, nodeName string, messageID string, message []byte, ttl time.Duration) error {
	ctx := context.Background()

	// Stocke le message temporairement avec TTL individuel
	key := "fluxmsg:" + messageID
	if err := rdb.Set(ctx, key, message, ttl).Err(); err != nil {
		return err
	}

	// Publie sur le canal pour diffusion immédiate
	channel := "flux:" + nodeName
	if err := rdb.Publish(ctx, channel, messageID).Err(); err != nil {
		return err
	}

	return nil
}

// SubscribeFlux s'abonne à un flux et renvoie les messages via un channel Go
func SubscribeFlux(rdb *redis.Client, nodeName string) (<-chan []byte, context.CancelFunc) {
	channel := "flux:" + nodeName
	ctx, cancel := context.WithCancel(context.Background())

	pubsub := rdb.Subscribe(ctx, channel)
	ch := make(chan []byte, 100) // buffer côté Go

	go func() {
		defer pubsub.Close()
		for msg := range pubsub.Channel() {
			messageID := msg.Payload
			// Récupère le message stocké temporairement
			data, err := rdb.Get(ctx, "fluxmsg:"+messageID).Bytes()
			if err == redis.Nil {
				continue // TTL déjà expiré
			} else if err != nil {
				log.Println("Erreur récupération flux message:", err)
				continue
			}
			ch <- data
		}
		close(ch)
	}()

	return ch, cancel
}

// ---------------- Cache ----------------

// SetCache ajoute un élément au cache
func (lru *LRUCache) SetCache(ctx context.Context, nodeName, elementID string, value []byte) error {
	key := "cache:" + nodeName
	if err := lru.rdb.HSet(ctx, key, elementID, value).Err(); err != nil {
		return err
	}
	lru.MarkUsed(nodeName, elementID)
	return nil
}

// GetCache lit un élément du cache
func (lru *LRUCache) GetCache(ctx context.Context, nodeName, elementID string) ([]byte, error) {
	key := "cache:" + nodeName
	val, err := lru.rdb.HGet(ctx, key, elementID).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err == nil {
		lru.MarkUsed(nodeName, elementID)
	}
	return val, err
}

// ---------------- Global ----------------

// GlobalStrategy est l’instance globale de stratégie LRU utilisée par toute l’app
var GlobalStrategy *LRUCache
```
et dapatation du nouveau système dans le `hub.go` :
```go 
package websocket

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"sync"

	"github.com/QuentinRegnier/nubo-backend/internal/cache"
	"github.com/gorilla/websocket"
)

// generateMessageID crée un ID unique pour chaque message
func generateMessageID() string {
	b := make([]byte, 8) // 8 octets → 16 caractères hex
	if _, err := rand.Read(b); err != nil {
		return "msg-fallback" // fallback si erreur improbable
	}
	return hex.EncodeToString(b)
}

// ---------------- Clients ----------------

type Client struct {
	conn *websocket.Conn
	send chan []byte
}

// ---------------- Hub ----------------

type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	mu         sync.Mutex

	channel string
}

// NewHub crée un nouveau Hub et lance l'écoute du flux Redis
func NewHub() *Hub {
	h := &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte),
		channel:    "nubo-websocket",
	}

	// Utilise la fonction SubscribeFlux pour recevoir les messages
	go h.listenFlux()
	return h
}

// listenFlux s'abonne au flux Redis et distribue les messages aux clients
func (h *Hub) listenFlux() {
	ch, cancel := cache.SubscribeFlux(cache.Rdb, h.channel)
	defer cancel()

	for msg := range ch {
		h.mu.Lock()
		for client := range h.clients {
			select {
			case client.send <- msg:
			default:
				close(client.send)
				delete(h.clients, client)
			}
		}
		h.mu.Unlock()
	}
}

// Run démarre la boucle principale du hub pour gérer l'inscription/désinscription et la diffusion
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Println("Client registered")

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				log.Println("Client unregistered")
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			// Publie le message sur le flux Redis avec TTL individuel (ex: 1s)
			messageID := generateMessageID() // fonction pour créer un ID unique
			err := cache.PushFluxWithTTL(cache.Rdb, h.channel, messageID, message, cache.DefaultFluxTTL)
			if err != nil {
				log.Println("Erreur PushFluxWithTTL:", err)
			}
		}
	}
}

// ---------------- Clients WS ----------------

// ReadPump lit les messages d’un client et les envoie au hub
func (c *Client) ReadPump(hub *Hub) {
	defer func() {
		hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break
		}

		// TODO: sauvegarder msg en base (Postgres/Mongo)

		// Envoie le message aux autres clients via le hub
		hub.broadcast <- msg
	}
}

// WritePump envoie les messages du hub au client
func (c *Client) WritePump() {
	for msg := range c.send {
		err := c.conn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			log.Println("Write error:", err)
			break
		}
	}
	c.conn.Close()
}
```
ajout également de la déclaration de l'observation de la RAM dans `main.go` :
```go
// ⚡ Initialiser la stratégie Redis
<cache.GlobalStrategy = cache.NewLRUCache(cache.Rdb)

// ⚡ Démarrer le watcher mémoire
// maxRAM = 0 => autodétection
// marge = 200 Mo de marge de sécurité
// interval = toutes les 2 secondes
cache.GlobalStrategy.StartMemoryWatcher(0, 200*1024*1024, 2*time.Second)
```