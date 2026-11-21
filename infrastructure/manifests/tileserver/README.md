# TileServer GL

Sert les tuiles vectorielles (vector tiles) au format MapLibre/MBTiles aux
clients mobile via `https://tiles.tissimah.kpeewu.dev/data/west-africa/{z}/{x}/{y}.pbf`.

Couvre les 4 pays cibles depuis le fichier `west-africa.pmtiles` (~1-3 Gi),
généré par le Job [tile-prep](../tile-prep/job.yaml) à partir de
`west-africa.osm.pbf` (produit par [osm-data-prep](../osm-data-prep)).

## Manifests

| Fichier | Rôle |
|---|---|
| `pvc.yaml` | PVC 5 Gi pour le `.pmtiles` |
| `configmap.yaml` | config TileServer (déclare le dataset west-africa) |
| `deployment.yaml` | TileServer GL v4.12, 2 réplicas, anti-affinity |
| `service.yaml` | ClusterIP `tileserver-gl:8080` |
| `ingress.yaml` | Ingress public `tiles.tissimah.kpeewu.dev` avec cache HTTP long |
| `hpa.yaml` | HPA 2-4 réplicas (CPU 60%) |
| `pdb.yaml` | PodDisruptionBudget minAvailable: 1 |

## Première initialisation

```bash
# 1. Le PVC osrm-data doit contenir west-africa.osm.pbf
# (le Job osm-data-prep doit avoir tourné).

# 2. Créer le PVC tileserver
kubectl apply -f infrastructure/manifests/tileserver/pvc.yaml

# 3. Lancer le Job de génération du .pmtiles (durée 30-60 min)
kubectl apply -f infrastructure/manifests/tile-prep/job.yaml
kubectl wait --for=condition=complete --timeout=2h job/tile-prep -n default
kubectl logs -n default job/tile-prep | tail

# 4. Déployer TileServer + Service + Ingress
kubectl apply -f infrastructure/manifests/tileserver/configmap.yaml \
              -f infrastructure/manifests/tileserver/deployment.yaml \
              -f infrastructure/manifests/tileserver/service.yaml \
              -f infrastructure/manifests/tileserver/hpa.yaml \
              -f infrastructure/manifests/tileserver/pdb.yaml \
              -f infrastructure/manifests/tileserver/ingress.yaml
```

## Vérification

```bash
# TileJSON metadata
curl https://tiles.tissimah.kpeewu.dev/data/west-africa.json

# Tuile vectorielle (zoom 13, autour de Lomé)
curl -I https://tiles.tissimah.kpeewu.dev/data/west-africa/13/4012/3851.pbf
# Attendu : 200, Content-Type: application/x-protobuf,
#           Cache-Control: public, max-age=604800, immutable
```

## Cache HTTP

L'Ingress configure :
```
Cache-Control: public, max-age=604800, immutable
```

Cela permet :
- Au client mobile de cacher localement chaque tuile pendant 7 jours
- À un CDN éventuel (Cloudflare, Bunny.net) de caché en edge

Quand on régénère le `.pmtiles` (cf. refresh hebdomadaire), les tuiles
existantes restent valides — leur contenu change peu d'une semaine à
l'autre. Si on veut invalider plus aggressivement, faire un `rollout
restart` de TileServer + ajouter un query param de version sur les URLs
côté client.

## Refresh

Quand `osm-data-prep` régénère `west-africa.osm.pbf` :

```bash
# 1. Re-lancer la génération du .pmtiles
kubectl delete job tile-prep -n default --ignore-not-found
kubectl apply -f infrastructure/manifests/tile-prep/job.yaml
kubectl wait --for=condition=complete --timeout=2h job/tile-prep -n default

# 2. Faire reloader TileServer (le PVC est déjà à jour, mais TileServer
# ouvre le .pmtiles au démarrage uniquement)
kubectl rollout restart deploy/tileserver-gl -n default
```

## Resources

- TileServer runtime : ~256 Mi RAM par pod, charge CPU faible (lecture .pmtiles).
- Job planetiler : 4-6 Gi RAM, multi-CPU, durée 30-60 min pour 4 pays.

Le bottleneck en prod sera la bande passante sortante (chaque session mobile
charge 50-200 tuiles au démarrage). À surveiller via metrics côté Ingress.
