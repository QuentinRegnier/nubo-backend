#!/bin/bash
set -e

echo "Choisissez le mode :"
select mode in "DEV" "PROD"; do
    case $mode in
        DEV)
            echo "🚀 Lancement en mode DEV"
            go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/main.go -d . --parseDependency --parseInternal
            docker compose \
              -f docker-compose.dev.yml \
              up -d --build --remove-orphans
            break
            ;;
        PROD)
            echo "🚀 Lancement en mode PROD"

            # Si un jour tu sépares WireGuard dans un compose dédié
            # cd wireguard
            # docker compose -f docker-compose.wg.yml up -d
            # cd ..

            docker compose \
              -f docker-compose.prod.yml \
              up -d --build --remove-orphans
            break
            ;;
    esac
done