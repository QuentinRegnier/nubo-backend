# ---------------------------------------------------------
# ÉTAPE 1 : Builder (On compile l'application)
# ---------------------------------------------------------
# On utilise ta version (1.24 ou 1.25), mais en version "alpine"
FROM golang:1.25-alpine AS builder

# Installation des outils C obligatoires pour "chai2010/webp"
# gcc & musl-dev : Pour compiler le C
# libwebp-dev    : Contient les headers (.h) nécessaires au build
RUN apk add --no-cache gcc musl-dev libwebp-dev

WORKDIR /app

# Gestion du cache des modules (pour que le build soit plus rapide les prochaines fois)
COPY go.mod go.sum ./
RUN go mod download

# Copie du code source
COPY . .

# Compilation
# CGO_ENABLED=1 est impératif car ta lib d'image utilise du code C
# -ldflags="-w -s" réduit la taille du binaire (enlève les infos de debug)
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o nubo cmd/main.go

# ---------------------------------------------------------
# ÉTAPE 2 : Runner (L'image finale de production)
# ---------------------------------------------------------
FROM alpine:latest

# On installe UNIQUEMENT la librairie d'exécution (pas les outils de dev)
# Sans ça, ton binaire ne pourra pas charger la lib webp au démarrage
RUN apk add --no-cache libwebp ca-certificates

WORKDIR /app

# 1. On récupère le binaire
COPY --from=builder /app/nubo .

# 2. On récupère le dossier docs (généré par swag)
COPY --from=builder /app/docs ./docs

# 3. On récupère le fichier HTML de Scalar
COPY --from=builder /app/docs.html .

# Création des dossiers nécessaires (si tu utilises le stockage local temporaire)
RUN mkdir -p /app/uploads

# Variables d'environnement par défaut (optionnel)
ENV GIN_MODE=release

EXPOSE 8080

CMD ["./nubo"]