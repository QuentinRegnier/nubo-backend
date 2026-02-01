#!/bin/bash
set -e

# Version compatible macOS et Linux pour mettre en majuscule
MODE=$(echo "$1" | tr '[:lower:]' '[:upper:]')

if [ -z "$MODE" ]; then
    echo "Choisissez le mode :"
    select opt in "DEV" "PROD"; do
        MODE=$opt
        break
    done
fi

case $MODE in
    DEV)
        echo "🚀 Lancement en mode DEV"
        go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/main.go -d . --parseDependency --parseInternal
        docker compose -f docker-compose.dev.yml up -d --build --remove-orphans
        ;;
    PROD)
        echo "🚀 Lancement en mode PROD"
        docker compose -f docker-compose.prod.yml up -d --build --remove-orphans
        ;;
    *)
        echo "Usage: ./launch.sh {DEV|PROD}"
        exit 1
        ;;
esac