# ==========================================
# 1. BASE (Le socle commun)
# ==========================================
# On part de la même image Go que tu as choisie
FROM golang:1.25-alpine AS base

# POURQUOI : On installe les outils C ici pour ne le faire qu'une seule fois.
# C'est l'étape la plus longue (gcc, musl-dev, libwebp-dev).
# Docker mettra cette étape en cache et ne la refera plus jamais tant que tu ne touches pas à cette ligne.
RUN apk add --no-cache gcc musl-dev libwebp-dev

WORKDIR /app

# POURQUOI : On installe "Air" ici.
# C'est l'outil qui va surveiller tes fichiers et redémarrer l'appli en 1 seconde.
RUN go install github.com/air-verse/air@latest

# POURQUOI : On télécharge les dépendances Go (go.mod/sum).
# En le faisant dans "base", le cache est partagé entre le mode Dev et le mode Prod.
COPY go.mod go.sum ./
RUN go mod download

# ==========================================
# 2. DEV (L'environnement de développement)
# ==========================================
FROM base AS dev

# POURQUOI : On ne fait PAS de "COPY . ." ici.
# En dev, ton code sera "monté" via le docker-compose (volumes).
# Ainsi, Air verra tes fichiers locaux changer instantanément.

# Commande de démarrage pour le dev : on lance Air
CMD ["air", "-c", ".air.toml"]

# ==========================================
# 3. BUILDER-PROD (La compilation pour la Prod)
# ==========================================
# On repart de "base", mais cette fois pour construire le binaire final
FROM base AS builder-prod

# POURQUOI : Ici, on COPIE le code source dans l'image.
# C'est nécessaire pour la prod car l'image doit être autonome (sans volume).
COPY . .

# Ta commande de compilation originale (optimisée et strippée)
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o nubo cmd/main.go

# ==========================================
# 4. PROD (L'image finale légère)
# ==========================================
# C'est ton étape finale actuelle, inchangée.
FROM alpine:latest AS prod

# On réinstalle la lib d'exécution (nécessaire pour l'image finale minimaliste)
RUN apk add --no-cache libwebp ca-certificates

WORKDIR /app

# On copie uniquement le binaire compilé depuis l'étape "builder-prod"
COPY --from=builder-prod /app/nubo .
COPY --from=builder-prod /app/docs ./docs
COPY --from=builder-prod /app/docs.html .

RUN mkdir -p /app/uploads
ENV GIN_MODE=release
EXPOSE 8080

CMD ["./nubo"]