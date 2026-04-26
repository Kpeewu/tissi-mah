# OSM Data Prep

Pipeline de préparation des données OpenStreetMap pour OSRM.

## Quoi

- **`scripts/geo/prepare-osm-data.sh`** (à la racine du repo) — script bash standalone
  utilisable en local ou dans n'importe quel container avec curl + osmium + osrm-*.
- **`job.yaml`** — Kubernetes Job one-shot. Télécharge les extracts Geofabrik des 4 pays,
  merge avec osmium, fait le pré-traitement OSRM (extract → partition → customize),
  écrit dans le PVC `osrm-data`. Durée typique : 2-4h.
- **`cronjob.yaml`** — Refresh hebdomadaire (dimanche 02:00 UTC). Scale OSRM à 0,
  régénère les données, puis l'opérateur scale OSRM à 1 (étape manuelle pour V1).

## Première initialisation

```bash
# 1. Créer le PVC
kubectl apply -f infrastructure/manifests/osrm/pvc.yaml

# 2. Lancer le Job de prep (le PVC est vide, OSRM n'est pas encore déployé)
kubectl apply -f infrastructure/manifests/osm-data-prep/job.yaml

# 3. Suivre la progression
kubectl logs -f -n dev job/osm-data-prep -c download-and-merge
kubectl logs -f -n dev job/osm-data-prep -c osrm-process

# 4. Attendre la fin
kubectl wait --for=condition=complete --timeout=4h job/osm-data-prep -n dev

# 5. Déployer OSRM
kubectl apply -f infrastructure/manifests/osrm/
```

## Refresh manuel

Si on veut re-générer les données entre les CronJobs hebdomadaires :

```bash
kubectl scale deploy/osrm-backend --replicas=0 -n dev
kubectl delete job osm-data-prep -n dev --ignore-not-found
kubectl apply -f infrastructure/manifests/osm-data-prep/job.yaml
kubectl wait --for=condition=complete --timeout=4h job/osm-data-prep -n dev
kubectl scale deploy/osrm-backend --replicas=1 -n dev
```

## Refresh automatique (CronJob)

```bash
kubectl apply -f infrastructure/manifests/osm-data-prep/cronjob.yaml
```

Le CronJob s'occupe du scale-down d'OSRM avant le refresh, mais **pas** du
scale-up post-refresh — l'opérateur doit vérifier le succès du Job puis
faire un `kubectl scale deploy/osrm-backend --replicas=1` manuellement.
On évite l'automatisation complète pour ne pas redémarrer OSRM si la
préparation a produit des données corrompues.

Pour automatiser entièrement la séquence, utiliser Argo Workflows ou
Tekton (hors scope V1).

## Resources

- **Phase download + merge** : 1-2 Gi RAM, peu de CPU (limité par la bande passante).
- **Phase OSRM** : 4-8 Gi RAM, multi-CPU (osrm-extract est très intensif).
- **Disque** : ~10 Gi sur le PVC (1 Go .osm.pbf + 5-7 Go .osrm).

## Limites connues

- **PVC RWO** : OSRM doit être stoppé pendant le Job. Cf. plan d'implémentation
  pour les options de migration vers RWX.
- **Pas de validation post-refresh** : si les données OSM étaient corrompues
  ou Geofabrik renvoie un fichier invalide, on perdrait l'ancien build. Mitigation
  V2 : sauvegarder l'ancienne version dans `/data/west-africa.osrm.bak` avant
  régénération + rollback automatique en cas d'échec.
