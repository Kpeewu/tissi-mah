# Bases de données TissiMah - VPS-dev

Ce répertoire contient les configurations Docker Compose pour les bases de données de tous les services TissiMah.

## 📋 Architecture

Chaque microservice a son propre docker-compose avec ses bases de données dédiées :

```
infrastructure/databases/
├── docker-compose-auth.yml      ✅ Auth Service (PostgreSQL + Redis)
├── docker-compose-user.yml      🔜 User Service (PostgreSQL + Redis + MongoDB)
├── docker-compose-trips.yml     🔜 Trips Service (PostgreSQL + Redis)
├── docker-compose-booking.yml   🔜 Booking Service (PostgreSQL + Redis)
├── docker-compose-payment.yml   🔜 Payment Service (PostgreSQL + Redis)
└── ...
```

### Structure des migrations

Les migrations SQL sont organisées par service avec des dossiers séparés :

```
services/auth-service/migrations/
├── up/                              # Migrations pour créer/modifier
│   ├── 000001_create_auth_table.sql
│   ├── 000002_add_email_index.sql
│   └── ...
└── down/                            # Migrations pour rollback
    ├── 000001_drop_auth_table.sql
    ├── 000002_remove_email_index.sql
    └── ...
```

**Important** :
- Seul le dossier `up/` est monté dans Docker (exécuté automatiquement au premier démarrage)
- Le dossier `down/` est pour la documentation et les rollbacks manuels
- Les fichiers sont exécutés par ordre alphabétique (d'où la numérotation)

### Réseau

Les conteneurs utilisent `network_mode: host`, ce qui signifie :
- PostgreSQL écoute directement sur `127.0.0.1:5432` du VPS
- Redis écoute directement sur `127.0.0.1:6379` du VPS
- Les services K3s peuvent se connecter via `127.0.0.1` ou l'IP publique du VPS

## 🚀 Démarrage rapide

### 1. Configuration initiale

```bash
# Copier le fichier d'exemple
cp .env.example .env

# Générer des mots de passe sécurisés
openssl rand -base64 32  # Pour AUTH_POSTGRES_PASSWORD
openssl rand -base64 32  # Pour AUTH_REDIS_PASSWORD

# Éditer le fichier .env
nano .env

# Variables à configurer :
# AUTH_POSTGRES_DB=auth_db               # Nom de la base de données
# AUTH_POSTGRES_USER=auth_user           # Utilisateur PostgreSQL
# AUTH_POSTGRES_PASSWORD=<mot_de_passe>  # Mot de passe généré
# AUTH_POSTGRES_PORT=5432                # Port PostgreSQL
# AUTH_POSTGRES_HOST=127.0.0.1           # Host PostgreSQL
# POSTGRES_HOST_AUTH_METHOD=scram-sha-256 # Méthode d'authentification
# AUTH_REDIS_PASSWORD=<mot_de_passe>     # Mot de passe Redis
# AUTH_REDIS_PORT=6379                   # Port Redis
```

### 2. Démarrer les bases de données pour auth-service

```bash
# Démarrer en mode détaché (background)
docker compose -f docker-compose-auth.yml up -d

# Vérifier que les conteneurs sont bien démarrés
docker compose -f docker-compose-auth.yml ps

# Voir les logs
docker compose -f docker-compose-auth.yml logs -f
```

### 3. Vérification

```bash
# Health check PostgreSQL
docker compose -f docker-compose-auth.yml exec postgres-auth pg_isready -U auth_user

# Health check Redis
docker compose -f docker-compose-auth.yml exec redis-auth redis-cli ping

# Connexion à PostgreSQL
docker compose -f docker-compose-auth.yml exec postgres-auth psql -U auth_user -d auth_db

# Connexion à Redis
docker compose -f docker-compose-auth.yml exec redis-auth redis-cli -a ${AUTH_REDIS_PASSWORD}
```

## 📚 Commandes utiles

### Gestion des conteneurs

```bash
# Démarrer
docker compose -f docker-compose-auth.yml up -d

# Arrêter
docker compose -f docker-compose-auth.yml down

# Redémarrer
docker compose -f docker-compose-auth.yml restart

# Voir les logs en temps réel
docker compose -f docker-compose-auth.yml logs -f

# Logs d'un service spécifique
docker compose -f docker-compose-auth.yml logs -f postgres-auth

# Statut des conteneurs
docker compose -f docker-compose-auth.yml ps

# Statistiques de ressources
docker stats tissimah-postgres-auth tissimah-redis-auth
```

### PostgreSQL

```bash
# Se connecter à psql
docker exec -it tissimah-postgres-auth psql -U auth_user -d auth_db

# Lister les bases de données
docker exec -it tissimah-postgres-auth psql -U auth_user -l

# Exécuter une requête SQL
docker exec -it tissimah-postgres-auth psql -U auth_user -d auth_db -c "SELECT version();"

# Voir les tables
docker exec -it tissimah-postgres-auth psql -U auth_user -d auth_db -c "\dt"

# Backup de la base de données
docker exec tissimah-postgres-auth pg_dump -U auth_user auth_db > backup-auth-$(date +%Y%m%d).sql

# Restore d'un backup
docker exec -i tissimah-postgres-auth psql -U auth_user auth_db < backup-auth-20250124.sql

# Voir les connexions actives
docker exec -it tissimah-postgres-auth psql -U auth_user -d auth_db -c "SELECT * FROM pg_stat_activity;"
```

### Redis

```bash
# Se connecter au CLI Redis
docker exec -it tissimah-redis-auth redis-cli -a ${AUTH_REDIS_PASSWORD}

# Vérifier la connexion
docker exec tissimah-redis-auth redis-cli -a ${AUTH_REDIS_PASSWORD} ping

# Voir les statistiques
docker exec tissimah-redis-auth redis-cli -a ${AUTH_REDIS_PASSWORD} INFO

# Nombre de clés
docker exec tissimah-redis-auth redis-cli -a ${AUTH_REDIS_PASSWORD} DBSIZE

# Lister toutes les clés (ATTENTION en production !)
docker exec tissimah-redis-auth redis-cli -a ${AUTH_REDIS_PASSWORD} KEYS "*"

# Supprimer toutes les clés (DANGER !)
docker exec tissimah-redis-auth redis-cli -a ${AUTH_REDIS_PASSWORD} FLUSHALL

# Voir l'utilisation mémoire
docker exec tissimah-redis-auth redis-cli -a ${AUTH_REDIS_PASSWORD} INFO memory
```

### Volumes

```bash
# Lister les volumes
docker volume ls | grep auth

# Inspecter un volume
docker volume inspect databases_postgres-auth-data

# Backup d'un volume
docker run --rm \
  -v databases_postgres-auth-data:/data \
  -v $(pwd):/backup \
  alpine tar czf /backup/postgres-auth-backup-$(date +%Y%m%d).tar.gz -C /data .

# Restore d'un volume
docker run --rm \
  -v databases_postgres-auth-data:/data \
  -v $(pwd):/backup \
  alpine tar xzf /backup/postgres-auth-backup-20250124.tar.gz -C /data

# ⚠️ DANGER : Supprimer un volume (perte de données !)
docker volume rm databases_postgres-auth-data
```

## 🔧 Configuration de auth-service dans K3s

Une fois les bases de données démarrées, configure auth-service pour s'y connecter :

### ConfigMap Kubernetes

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: auth-service-config
  namespace: default
data:
  # PostgreSQL
  DB_HOST: "127.0.0.1"  # ou l'IP publique du VPS
  DB_PORT: "5432"       # Correspond à AUTH_POSTGRES_PORT
  DB_NAME: "auth_db"    # Correspond à AUTH_POSTGRES_DB
  DB_USER: "auth_user"  # Correspond à AUTH_POSTGRES_USER
  DB_SSLMODE: "disable"  # En dev, en prod utilisez "require"
  
  # Redis
  REDIS_HOST: "127.0.0.1"  # ou l'IP publique du VPS
  REDIS_PORT: "6379"       # Correspond à AUTH_REDIS_PORT
  REDIS_DB: "0"
```

### Secret Kubernetes

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: auth-service-secrets
  namespace: default
type: Opaque
stringData:
  # PostgreSQL password (même valeur que AUTH_POSTGRES_PASSWORD dans .env)
  DB_PASSWORD: "your_postgres_password_here"
  
  # Redis password (même valeur que AUTH_REDIS_PASSWORD dans .env)
  REDIS_PASSWORD: "your_redis_password_here"
```

### Création des secrets

```bash
# Créer le secret depuis le terminal
kubectl create secret generic auth-service-secrets \
  --from-literal=DB_PASSWORD="${AUTH_POSTGRES_PASSWORD}" \
  --from-literal=REDIS_PASSWORD="${AUTH_REDIS_PASSWORD}" \
  --namespace=default

# Vérifier
kubectl get secret auth-service-secrets -o yaml
```

## 🔒 Sécurité

### Mots de passe

```bash
# Générer des mots de passe forts
openssl rand -base64 32
pwgen 32 1
uuidgen

# NE JAMAIS commiter le fichier .env dans Git !
# Vérifiez que .gitignore contient :
echo ".env" >> .gitignore
echo "*.env" >> .gitignore
echo "!.env.example" >> .gitignore
```

### Rotation des mots de passe

```bash
# 1. Modifier .env avec les nouveaux mots de passe
nano .env

# 2. Redémarrer les conteneurs
docker compose -f docker-compose-auth.yml restart

# 3. Mettre à jour les secrets K8s
kubectl delete secret auth-service-secrets
kubectl create secret generic auth-service-secrets \
  --from-literal=DB_PASSWORD="${AUTH_POSTGRES_PASSWORD}" \
  --from-literal=REDIS_PASSWORD="${AUTH_REDIS_PASSWORD}"

# 4. Redémarrer auth-service
kubectl rollout restart deployment auth-service
```

### Firewall (UFW)

Si tu actives UFW sur ton VPS, assure-toi d'autoriser les connexions K3s :

```bash
# Autoriser le réseau K3s à accéder aux BDs
ufw allow from 10.42.0.0/16 to any port 5432 proto tcp
ufw allow from 10.42.0.0/16 to any port 6379 proto tcp

# Bloquer l'accès externe
ufw deny from any to any port 5432 proto tcp
ufw deny from any to any port 6379 proto tcp
```

## 🐛 Dépannage

### PostgreSQL ne démarre pas

```bash
# Voir les logs
docker compose -f docker-compose-auth.yml logs postgres-auth

# Vérifier que le port 5432 est libre
sudo netstat -tlnp | grep 5432

# Si le port est occupé, identifier le processus
sudo lsof -i :5432

# Supprimer les données corrompues (DANGER: perte de données !)
docker compose -f docker-compose-auth.yml down -v
docker compose -f docker-compose-auth.yml up -d
```

### Redis ne démarre pas

```bash
# Voir les logs
docker compose -f docker-compose-auth.yml logs redis-auth

# Vérifier que le port 6379 est libre
sudo netstat -tlnp | grep 6379

# Test de connexion
telnet 127.0.0.1 6379
```

### Problème de connexion depuis K3s

```bash
# Depuis un pod K3s, tester la connexion PostgreSQL
kubectl run -it --rm debug --image=postgres:16-alpine --restart=Never -- \
  psql -h 127.0.0.1 -U auth_user -d auth_db

# Tester la connexion Redis
kubectl run -it --rm debug --image=redis:7-alpine --restart=Never -- \
  redis-cli -h 127.0.0.1 -a ${AUTH_REDIS_PASSWORD} ping

# Vérifier la résolution DNS et réseau
kubectl run -it --rm debug --image=alpine --restart=Never -- sh
# Puis dans le shell :
ping -c 3 127.0.0.1
telnet 127.0.0.1 5432
```

### Erreur "password authentication failed"

```bash
# Vérifier que les mots de passe correspondent entre .env et K8s secrets
source .env
echo $AUTH_POSTGRES_PASSWORD

kubectl get secret auth-service-secrets -o jsonpath='{.data.DB_PASSWORD}' | base64 -d
echo

# Réinitialiser le mot de passe PostgreSQL
docker exec -it tissimah-postgres-auth psql -U postgres -c \
  "ALTER USER auth_user WITH PASSWORD 'nouveau_mot_de_passe';"
```

## 📊 Monitoring

### Logs centralisés

```bash
# Tous les logs en temps réel
docker compose -f docker-compose-auth.yml logs -f --tail=100

# Uniquement PostgreSQL
docker compose -f docker-compose-auth.yml logs -f postgres-auth

# Uniquement Redis
docker compose -f docker-compose-auth.yml logs -f redis-auth

# Logs avec horodatage
docker compose -f docker-compose-auth.yml logs -f -t
```

### Métriques

```bash
# Utilisation CPU/RAM en temps réel
docker stats tissimah-postgres-auth tissimah-redis-auth

# Espace disque des volumes
docker system df -v | grep auth

# Connexions PostgreSQL actives
docker exec tissimah-postgres-auth psql -U auth_user -d auth_db -c \
  "SELECT count(*) FROM pg_stat_activity WHERE state = 'active';"

# Mémoire Redis
docker exec tissimah-redis-auth redis-cli -a ${AUTH_REDIS_PASSWORD} INFO memory | grep used_memory_human
```

## 🔄 Migration vers d'autres services

Quand tu voudras ajouter user-service, trips-service, etc. :

```bash
# 1. Copier le template
cp docker-compose-auth.yml docker-compose-user.yml

# 2. Modifier les noms de conteneurs et volumes
sed -i 's/auth/user/g' docker-compose-user.yml

# 3. Changer les ports si nécessaire
# PostgreSQL: 5433 au lieu de 5432
# Redis: 6380 au lieu de 6379

# 4. Ajouter les variables d'environnement dans .env
echo "USER_POSTGRES_PASSWORD=$(openssl rand -base64 32)" >> .env
echo "USER_REDIS_PASSWORD=$(openssl rand -base64 32)" >> .env

# 5. Démarrer
docker compose -f docker-compose-user.yml up -d
```

## 📖 Ressources

- [Documentation PostgreSQL](https://www.postgresql.org/docs/)
- [Documentation Redis](https://redis.io/docs/)
- [Docker Compose Reference](https://docs.docker.com/compose/compose-file/)
- [Best practices Docker](https://docs.docker.com/develop/dev-best-practices/)

## 🆘 Support

En cas de problème :
1. Vérifier les logs : `docker compose logs -f`
2. Vérifier le statut : `docker compose ps`
3. Vérifier les ressources : `docker stats`
4. Consulter ce README
5. Demander de l'aide à l'équipe