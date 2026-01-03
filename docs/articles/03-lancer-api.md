# Lancer l'API Nubo

Vous pouvez lancer l'API en **mode développement** ou **mode production**.  
Un script `run.sh` simplifie le choix entre DEV et PROD.

## Méthode manuelle

### Mode DEV

```bash
docker compose -f docker-compose.dev.yml up --build
```

### Mode PORD

```bash
docker compose -f docker-compose.prod.yml up -d --build
```

### Méthode simplifiée avec script

1.	Lancer le script :
```bash
./launch.sh
```

2.	Choisir avec les flèches ou tabulation :

- DEV → code monté, hot-reload via Air
- PROD → binaire compilé, plus léger, sécurisé

3.	Le script lance automatiquement :

- L’API correspondante (api1 + api2)
- Si mode PROD, demander à activer WireGuard pour accéder aux outils internes

