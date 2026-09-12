# ==============================================================================
# MAKEFILE NUBO V12 - GESTION D'INFRASTRUCTURE ET DE DÉVELOPPEMENT
# ==============================================================================

# Variables par défaut
COMPOSE_DEV=-f docker-compose.dev.yml
COMPOSE_PROD=-f docker-compose.prod.yml

.PHONY: docs dev prod down run-db run-api run-worker run-redis clean

# ------------------------------------------------------------------------------
# 1. DÉVELOPPEMENT LOCAL (Remplace l'ancien launch.sh)
# ------------------------------------------------------------------------------

# Génère la documentation Swagger proprement
docs:
	@echo "📚 Génération de la documentation Swagger..."
	go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/main.go -d . --parseDependency --parseInternal

# Lance tout l'environnement de DEV (DB + API + Cache) sur ton PC
dev: docs
	@echo "🚀 Lancement en mode DEV (Full Stack)..."
	docker compose $(COMPOSE_DEV) --profile all up -d --build --remove-orphans

# Lance l'environnement de PROD complet (Monolithique - Serveur Unique)
prod: docs
	@echo "🚀 Lancement en mode PROD (Full Stack)..."
	docker compose $(COMPOSE_PROD) --profile all up -d --build --remove-orphans

# Coupe tous les conteneurs
down:
	@echo "🛑 Arrêt de tous les conteneurs..."
	docker compose $(COMPOSE_DEV) --profile all down
	docker compose $(COMPOSE_PROD) --profile all down

# ------------------------------------------------------------------------------
# 2. DÉPLOIEMENT DISTRIBUÉ (SCALING HORIZONTAL SUR SERVEURS MULTIPLES)
# ------------------------------------------------------------------------------

# Sur le Serveur 1 (Le Master) : Ne lance que les bases de données
run-db:
	@echo "🗄️ Lancement du Rôle: BASE DE DONNÉES..."
	docker compose $(COMPOSE_PROD) --profile db-master up -d --build

# Sur le Serveur 2 : Ne lance que les APIs Go (Scalable à l'infini)
run-api:
	@echo "🌐 Lancement du Rôle: API SERVER..."
	docker compose $(COMPOSE_PROD) --profile api up -d --build

# Sur le Serveur 3 : Ne lance que les Workers (Calcul asynchrone)
run-worker:
	@echo "⚙️ Lancement du Rôle: WORKERS..."
	docker compose $(COMPOSE_PROD) --profile worker up -d --build

# Sur le Serveur 4 : Ne lance qu'un nœud Redis
run-redis:
	@echo "🧠 Lancement du Rôle: REDIS NODE..."
	docker compose $(COMPOSE_PROD) --profile redis-node up -d --build

# Nettoyage total du système Docker (Prudence !)
clean: down
	docker system prune -a --volumes