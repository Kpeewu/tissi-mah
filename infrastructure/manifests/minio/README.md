# MinIO — Déploiement K8s

Stockage S3-compatible pour les documents KYC, photos profil et pièces véhicules.  
Accessible publiquement via `https://storage.tissimah.kpeewu.dev` (HTTPS, TLS Let's Encrypt).

## Architecture

```
Client → HTTPS → [Nginx système / K8s Ingress] → HTTP → MinIO pod:9000
                  (TLS termination, Host préservé)
```

Le `Host: storage.tissimah.kpeewu.dev` est impérativement transmis à MinIO pour que la validation HMAC des URL présignées fonctionne.

## Déploiement VPS dev (K3s)

### 1. Créer les secrets Kubernetes

```bash
kubectl create secret generic minio-credentials \
  --from-literal=MINIO_ROOT_USER=minioadmin \
  --from-literal=MINIO_ROOT_PASSWORD=<mot-de-passe-fort> \
  --from-literal=FILESVC_ACCESS_KEY=filesvc \
  --from-literal=FILESVC_SECRET_KEY=<mot-de-passe-service-account> \
  -n default
```

> Ne jamais committer les vraies valeurs dans `secret.yaml`.

### 2. Appliquer les manifests K8s

```bash
kubectl apply -f infrastructure/manifests/minio/pvc.yaml
kubectl apply -f infrastructure/manifests/minio/deployment.yaml
kubectl apply -f infrastructure/manifests/minio/service.yaml
kubectl apply -f infrastructure/manifests/minio/networkpolicy.yaml
```

Attendre que MinIO soit Ready :
```bash
kubectl rollout status deployment/minio -n default
```

### 3. Initialiser le bucket et le compte de service

```bash
kubectl apply -f infrastructure/manifests/minio/init-job.yaml
kubectl logs -f job/minio-init -n default
```

Le job crée :
- Bucket `tissi-mah-files` (privé)
- Policy `file-service-policy` (GetObject, PutObject, DeleteObject sur le bucket uniquement)
- Compte de service `filesvc` avec cette policy

### 4. Configurer le DNS

Créer un enregistrement A :
```
storage.tissimah.kpeewu.dev → <IP du VPS>
```

### 5. Installer la config Nginx et obtenir le certificat TLS

```bash
# Copier la config Nginx
sudo cp infrastructure/manifests/minio/nginx-storage.conf \
  /etc/nginx/sites-available/storage.tissimah.kpeewu.dev
sudo ln -s /etc/nginx/sites-available/storage.tissimah.kpeewu.dev \
  /etc/nginx/sites-enabled/

# Vérifier la syntaxe
sudo nginx -t

# Obtenir le certificat Let's Encrypt (modifie automatiquement le fichier nginx)
sudo certbot --nginx -d storage.tissimah.kpeewu.dev

# Recharger Nginx
sudo systemctl reload nginx
```

### 6. Mettre à jour les secrets GitHub Actions

Dans les Settings → Secrets du dépôt GitHub :

| Secret | Valeur |
|--------|--------|
| `FILE_S3_ACCESS_KEY` | `filesvc` |
| `FILE_S3_SECRET_KEY` | le mot de passe du compte de service |
| `FILE_S3_ENDPOINT` | `http://minio:9000` (endpoint interne K8s) |
| `FILE_S3_PUBLIC_ENDPOINT` | `https://storage.tissimah.kpeewu.dev` |

## Déploiement EKS prod

Sur EKS, utiliser `ingress.yaml` à la place du Nginx système :

```bash
kubectl apply -f infrastructure/manifests/minio/ingress.yaml
```

Le cert-manager génère automatiquement le certificat TLS via `letsencrypt-prod`.

## Vérification

```bash
# Tester que l'endpoint HTTPS est joignable
curl -I https://storage.tissimah.kpeewu.dev/minio/health/live

# Tester une URL présignée (doit retourner 200)
# Générer via file-service, copier l'URL dans le navigateur → document visible

# Vérifier que HTTP redirige vers HTTPS
curl -I http://storage.tissimah.kpeewu.dev/
```

## Administration MinIO

La console MinIO (port 9001) n'est jamais exposée publiquement.  
Accès via port-forward :

```bash
kubectl port-forward deployment/minio 9001:9001 -n default
# Ouvrir http://localhost:9001 — login : MINIO_ROOT_USER / MINIO_ROOT_PASSWORD
```
