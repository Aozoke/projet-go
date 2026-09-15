# Roadmap WasmRedis

Objectif : une version simple, fonctionnelle et explicable de bout en bout.

## Termine

- setup Go, tests et compilation WASM
- parser `SET`, `GET`, `DELETE`, `ALL`, `GET WHERE` et TTL `EX`
- state en RAM et buffer d'ecritures dans le moteur Go
- batch execute dans l'ordre
- index inverse pour `equals`
- B-Tree pour `>`, `>=`, `<` et `<=`
- TTL lazy, balayage actif et suppression persistante
- Worker avec acces OPFS exclusif
- AOF, snapshot, compaction et restore exact des TTL
- SDK TypeScript fonctionnel, generique et valide par Zod
- React CRUD et filtres
- virtual scroll maison
- abonnement et render granulaire par ligne
- dataset 100 000 fourni, sans chargement par defaut
- tests Go avec race detector et tests TypeScript
- benchmarks moteur, batch, restore et FPS
- README, rapport de benchmark et presentation de 10 slides

## Verifications faites

- une cle revient apres flush AOF et rechargement
- 100 entrees reviennent apres snapshot et compaction
- une cle TTL disparait et produit un `DELETE` persistant
- `value >= 50` renvoie 50 resultats apres le seed 100
- 29 lignes DOM maximum pour le calcul avec 100 000 entrees
- une ligne editee passe de 1 a 2 renders, l'autre reste a 1
- aucun message d'erreur pendant le parcours navigateur final

## Bonus possibles plus tard

- synchronisation de plusieurs onglets avec BroadcastChannel et Web Locks
- suppression B-Tree sans reconstruction
- format binaire pour reduire la taille de l'AOF et du snapshot
- test complet du seed 100 000 sur une machine plus puissante
