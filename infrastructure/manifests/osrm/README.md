# OSRM Backend

Routing engine self-hosted, appelé par `geolocation-service` via `http://osrm-backend:5000`.

Couvre les 4 pays cibles : Togo, Ghana, Bénin, Burkina Faso.

## Manifests

| Fichier | Rôle |
|---|---|
| `pvc.yaml` | PersistentVolumeClaim 10 Gi pour les fichiers `.osrm` pré-générés |
| `deployment.yaml` | OSRM v5.27.1 en mode MLD, écoute sur :5000 |
| `service.yaml` | ClusterIP `osrm-backend:5000` (interne uniquement) |
| `pdb.yaml` | PodDisruptionBudget minAvailable: 1 |
| `hpa.yaml` | HPA inactif en V1 (RWO PVC), placeholder pour scaling futur |

## Ordre de déploiement

```bash
# 1. Créer le PVC
kubectl apply -f infrastructure/manifests/osrm/pvc.yaml

# 2. Lancer le Job de prep des données OSM (cf. infrastructure/manifests/osm-data-prep/)
#    Le Job télécharge les extracts Geofabrik, fait le merge et le pré-traitement
#    (osrm-extract → partition → customize). Durée ~2-4h.
kubectl apply -f infrastructure/manifests/osm-data-prep/job.yaml
kubectl wait --for=condition=complete --timeout=4h job/osm-data-prep -n dev

# 3. Une fois le Job terminé, déployer OSRM
kubectl apply -f infrastructure/manifests/osrm/

# 4. Vérifier
kubectl port-forward svc/osrm-backend 5000:5000 -n dev &
curl 'http://localhost:5000/route/v1/driving/1.2228,6.1319;0.6266,6.9269?overview=false'
# Attendu : { "code": "Ok", "routes": [...] }
```

## Scaling

**V1 : 1 réplique uniquement.**

La storageClass par défaut de K3s (`local-path`) est RWO — un seul pod peut
monter le PVC à la fois. Pour passer à plusieurs répliques OSRM (utile sous
charge), deux options :

### Option A — PVC ReadWriteMany (préféré)

Provisionner un storageClass RWX sur le cluster :
- NFS (server externe, ou nfs-subdir-external-provisioner)
- Longhorn avec `rwx` enabled

Puis modifier `pvc.yaml` :
```yaml
spec:
  accessModes: [ReadWriteMany]
  storageClassName: longhorn-rwx   # ou nfs
```

Puis bumper `replicas: N` dans `deployment.yaml` et `min/maxReplicas` dans `hpa.yaml`.

### Option B — initContainer copy depuis un PVC source

Garder un PVC source (rempli par le Job) en RWO + chaque pod OSRM a son propre
`emptyDir` rempli au démarrage par un initContainer qui clone depuis le source
via un sidecar `osrm-data-source` exposé en ClusterIP HTTP/rsync.

Cette option ajoute ~30-60s de bootstrap par pod et duplique le stockage,
mais ne nécessite pas de RWX. Pertinent si Longhorn n'est pas disponible.

## Resources

OSRM en mode MLD consomme ~2-4 Gi de RAM pour les 4 pays cibles. Les requests/limits
dans `deployment.yaml` reflètent ce profil. Sur un VPS avec moins de 8 Gi libres,
réduire à `requests: 1Gi, limits: 3Gi` est risqué — mieux vaut cap au niveau
applicatif (semaphore côté geolocation-service).

## Refresh des données

Les données OSM se périment lentement (mise à jour quotidienne par OSM, mais
les routes principales bougent peu). Plan d'implémentation : CronJob hebdomadaire
qui rejoue le pipeline de prep (cf. `infrastructure/manifests/osm-data-prep/cronjob.yaml`).
