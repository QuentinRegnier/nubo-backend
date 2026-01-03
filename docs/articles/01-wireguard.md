# Lancer WireGuard & se connecter aux outils d'administration

Pour sécuriser l'accès aux interfaces d'administration (MinIO, PGAdmin, Mongo Express, Redis Commander), utilisez WireGuard.

## Étapes

### A.Lancer le serveur WireGuard en Docker :

```bash
docker compose up -d wireguard
```
### B.Installer le client WireGuard sur votre machine :

- Mac : Installer l’app WireGuard depuis le Mac App Store. Importer le fichier .conf.
- Linux : sudo apt install wireguard (ou votre gestionnaire), importer le .conf.
- Windows : Installer WireGuard depuis le site officiel, importer le .conf ou QR code.
Se connecter : activer le tunnel WireGuard sur votre machine. Vous êtes maintenant sur le réseau sécurisé interne.

### C.Accès aux interfaces internes :
- PGAdmin : http://nubo_pgadmin:80
- Redis Commander : http://nubo_redis_commander:8081
- Mongo Express : http://nubo_mongo_express:8081
- MinIO Console : http://nubo_minio:9001