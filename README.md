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