# Nominatim Backend

Geocoding self-hosted (texte ↔ coordonnées) basé sur l'image
[mediagis/nominatim](https://github.com/mediagis/nominatim-docker), couvrant
les 4 pays cibles (Togo, Ghana, Bénin, Burkina Faso).

Appelé par `geolocation-service` via `http://nominatim-backend:8080`.

## Manifests

| Fichier | Rôle |
|---|---|
| `pvc.yaml` | PersistentVolumeClaim 15 Gi pour Postgres (index Nominatim) |
| `deployment.yaml` | Nominatim 4.4 + Postgres + PostGIS, écoute sur :8080 |
| `service.yaml` | ClusterIP `nominatim-backend:8080` (interne uniquement) |
| `pdb.yaml` | PodDisruptionBudget minAvailable: 1 |
| `secrets.yaml.example` | Template pour le secret Postgres (à copier en secrets.yaml) |

## Première initialisation

```bash
# 1. Créer le secret depuis le template
cp infrastructure/manifests/nominatim/secrets.yaml.example /tmp/nominatim-secret.yaml
# Éditer /tmp/nominatim-secret.yaml pour mettre un vrai password
kubectl apply -f /tmp/nominatim-secret.yaml

# 2. Créer le PVC
kubectl apply -f infrastructure/manifests/nominatim/pvc.yaml

# 3. Pré-requis : le Job osm-data-prep doit avoir produit /data/west-africa.osm.pbf
# dans le PVC osrm-data (Nominatim le réutilise via mount readOnly).

# 4. Déployer
kubectl apply -f infrastructure/manifests/nominatim/deployment.yaml \
              -f infrastructure/manifests/nominatim/service.yaml \
              -f infrastructure/manifests/nominatim/pdb.yaml

# 5. Premier démarrage : ~1-2h pour importer les 4 pays.
# Le pod ne sera pas Ready pendant cette période.
kubectl logs -f -n dev deploy/nominatim-backend
# Attendre les logs "INFO  Database setup completed" puis "Started API"
```

## Vérification

```bash
kubectl port-forward -n dev svc/nominatim-backend 8080:8080
curl 'http://localhost:8080/search?q=Marché+de+Lomé&format=json&limit=3'
curl 'http://localhost:8080/reverse?lat=6.13&lon=1.22&format=json'
```

## Refresh des données

Identique à OSRM : la régénération hebdomadaire du `west-africa.osm.pbf` par
`osm-data-prep/cronjob.yaml` ne suffit PAS — Nominatim doit aussi réimporter.
Pour V1, le refresh Nominatim est **manuel** :

```bash
# Stopper Nominatim
kubectl scale deploy/nominatim-backend --replicas=0 -n dev

# Wipe l'ancienne base Postgres
kubectl delete pvc nominatim-data -n dev
kubectl apply -f infrastructure/manifests/nominatim/pvc.yaml

# Redémarrer (réimport ~1-2h)
kubectl scale deploy/nominatim-backend --replicas=1 -n dev
```

L'automatisation hebdomadaire est volontairement reportée à V2 — un Nominatim
HS pendant 1-2h hebdomadaire serait inacceptable. La bonne solution V2 est
soit la réplication continue (`REPLICATION_URL=...minute`), soit un blue/green
avec deux Postgres alternés.

## Resources

- Phase d'import : 4-8 Gi RAM, multi-CPU (osm2pgsql intensif).
- Runtime : 2-4 Gi RAM, charge CPU faible (Postgres optimisé pour lecture).

Si le VPS est limité à 8 Gi total, lancer Nominatim avec `replicas: 0` pendant
les imports OSRM puis l'inverse, pour ne pas saturer la mémoire.
