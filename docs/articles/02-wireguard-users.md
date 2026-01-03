# Ajouter ou retirer un utilisateur WireGuard

## Ajouter un utilisateur

### A.Sur le serveur Docker WireGuard :

```bash
docker compose exec wireguard wg genkey | tee /etc/wireguard/peers/<peer_name>.key
docker compose exec wireguard wg pubkey < /etc/wireguard/peers/<peer_name>.key
```

### B.Sur le client :

Ajouter le peer dans le fichier de configuration WireGuard (wg0.conf)

```ini
[Peer]
PublicKey = <clé_publique>
AllowedIPs = 10.13.13.<dernier_octet>/32
```

### C.Redémarrer le serveur WireGuard :
```bash
docker compose restart wireguard
```

## Retirer un utilisateur
1. Supprimer le bloc [Peer] correspondant dans wg0.conf
2. Redémarrer le serveur WireGuard

## Installation WireGuard sur machine personnelle

- Mac : App Store → Installer → Importer .conf
- Linux : sudo apt install wireguard → Importer .conf
- Windows : Installer depuis le site officiel → Importer .conf ou QR code