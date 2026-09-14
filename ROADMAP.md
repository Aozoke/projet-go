# Roadmap WasmRedis

Objectif : garder une version qui fonctionne et que l'on peut expliquer.

## Fait

- setup Go et tests
- parser `SET`, `GET`, `DELETE`, `ALL`, `GET WHERE`
- moteur en RAM
- batch
- AOF, flush, snapshot et restore OPFS
- TTL lazy et balayage actif
- index `equals`
- B-Tree pour les plages numeriques
- Go compile en WASM
- Web Worker
- SDK TypeScript et query builder
- validation Zod dans le SDK et validation Go dans le parser
- UI React CRUD et filtres
- seed configurable, limite a 100 par defaut
- virtual scroll maison
- store externe et abonnement par ligne
- benchmark natif Go
- benchmark p50 / p95 dans le navigateur
- configuration `.env`

## Encore a ameliorer

- mesurer le FPS du scroll dans le navigateur
- mesurer le restore snapshot seul puis snapshot + AOF
- tester sur un plus gros ordinateur avec 100 000 entrees
- optimiser la mise a jour du B-Tree sans le reconstruire
- typer un schema de documents plus riche que `key -> value`
- completer le rapport de benchmark avec les mesures finales
- terminer la presentation

## Ordre conseille

1. Tester chaque bouton de l'interface.
2. Essayer une cle avec un TTL de 5 secondes.
3. Faire `Seed 100`, puis filtrer `value >= 50`.
4. Modifier une ligne et regarder son compteur orange.
5. Lancer le bouton `Benchmark` et noter les valeurs.
6. Recharger la page pour verifier le restore OPFS.
7. Reprendre ensuite la presentation, a partir du code reel.
