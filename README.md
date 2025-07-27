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

## 🔨 ÉTAPES DE DÉVELOPPEMENT

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

## 🌐 Création du repo GitHub

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

## 🛠️ Plan d’action clair (prochaine phase)

✅ Étape 1 — Ajouter les routes REST : Login, Signup, Post
✅ Étape 2 — Ajouter le WebSocket en Go
✅ Étape 3 — Brancher PostgreSQL, MongoDB, Redis
✅ Étape 4 — Ajouter l’authentification JWT
✅ Étape 5 — Déployer sur un serveur Linux (Docker Compose + .env)
✅ ÉTAPE 1 — Créer les routes REST de base

---

## 🎯 Objectif
### Créer des routes :

- `POST /signup` → créer un utilisateur
- `POST /login` → connecter (renvoyer JWT, à faire en Étape 4)
- `GET /posts` → récupérer les posts
- `POST /posts` → créer un post

## 📁 Organisation recommandée
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

## 🚀 Étape WebSocket : créer un serveur WebSocket basique en Go avec Gin + Gorilla WebSocket
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

### Code à ajouter `internal/websocket/hub.go`
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

### Modifier le handler WebSocket `internal/websocket/handler.go`
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

### Test :
- Lance l’API
- Connecte plusieurs clients au /ws
- Envoie un message d’un client → Tous les clients reçoivent le message

---

## 6. Intégration Redis Pub/Sub

Pourquoi ?

Pour permettre à plusieurs instances de ton backend (scalées horizontalement) de communiquer, diffuser les messages WS entre elles.

### Ajouter Redis Pub/Sub dans le hub

Installer Redis client déjà fait avec `go-redis/redis/v8`

### Ajouter un fichier `internal/cache/redis.go`
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

### Modifie `cmd/main.go` pour initialiser Redis
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

### Modifier le hub pour utiliser Redis Pub/Sub

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

## 7. Connecter le WebSocket à la base (exemple notifications)

### Exemple rapide

Dans Client.ReadPump(), à chaque message reçu, tu peux enregistrer ou traiter en DB.

### Exemple dans `internal/websocket/hub.go` ReadPump :
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

## 8. Protéger le WS avec JWT

### Ajouter middleware d’authentification JWT
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

### Protéger la route WebSocket

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

## ✅ PRÉREQUIS AVANT DE TESTER

- Redis tourne bien (docker-compose up)
- Ton backend Go est lancé (go run cmd/main.go)
Tu as un JWT valide (car le WS est protégé maintenant) — on en génère un dans l'étape 1 👇

---

## 🧪 1. Générer un JWT de test

### Ajoute un petit endpoint temporaire dans internal/api/routes.go juste pour tester :
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

## 🧪 2. Se connecter avec websocat + JWT

### Copie le token et lance cette commande dans le terminal :
```bash
websocat ws://localhost:8081/ws -H 'Authorization: Bearer Remplace TON_JWT_ICI'
```
Remplace TON_JWT_ICI par le vrai token.

### ✅ Tu dois voir dans les logs :
```bash
Utilisateur connecté (userID): user123
Client registered
```

---

## 🧪 3. Tester le broadcast

- Ouvre deux terminaux avec cette même commande websocat (et le même token).
- Tape un message dans l’un. Tu dois le recevoir dans les deux.

### 💬 Exemple :
```bash
> Salut à tous !
```

🟢 Les deux clients doivent afficher Salut à tous !.

---

## 🧪 4. Tester le Redis Pub/Sub (scalabilité)

### Étapes :
- Lance une deuxième instance de ton backend (dans un autre terminal) :
```bash
go run cmd/main.go
```
- Connecte websocat à chaque instance :
Instance 1 : port `8080`
Instance 2 : change à la volée : `r.Run(":8081")`, connecte à `ws://localhost:8081/ws`
- Envoie un message sur une instance, il doit apparaître sur toutes → preuve que Redis propulse les messages entre les processus backend.

---

## 🧪 5. Vérifie le JWT en cas d'erreur

Si tu connectes sans Authorization, tu dois recevoir :
```bash
{"error":"Authorization header manquant"}
```
Ou :
```bash
{"error":"Token invalide"}
```

