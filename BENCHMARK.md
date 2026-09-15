# Benchmark WasmRedis

Mesures realisees le 15 septembre 2026. Elles donnent un ordre de grandeur et
une methode reproductible, pas une promesse identique sur toutes les machines.

## Machine et methode

- CPU : Intel Core i5-12450H
- systeme : Linux amd64 sous WSL
- Go : 1.22.2
- navigateur : Chrome Headless Shell 153
- viewport : 1440 x 900
- seed navigateur : 100 entrees
- benchmark navigateur : 30 iterations

Commande Go :

```bash
go test -bench=. -benchmem -benchtime=300ms -count=1 ./internal/redis
```

Le benchmark navigateur est lance avec le bouton `Benchmark`. Chaque latence
est mesuree avec `performance.now()`, puis triee pour calculer p50 et p95.

## Moteur Go natif

| Mesure | Resultat |
|---|---:|
| SET | 454 ns/op |
| GET par cle | 51,5 ns/op |
| Batch de 4 GET | 1 243 ns/op |
| Restore snapshot, 10 000 entrees | 10,23 ms |
| Restore snapshot + 1 000 operations AOF | 18,61 ms |

### GET WHERE selon la taille

| Filtre | 100 | 1 000 | 10 000 |
|---|---:|---:|---:|
| equals | 219 ns | 190 ns | 221 ns |
| contains | 4,85 us | 50,35 us | 457,75 us |
| range, environ 50 % des resultats | 10,39 us | 178,19 us | 1,66 ms |
| range selective, 1 resultat | 261 ns | 259 ns | 326 ns |

`contains` scanne volontairement la base. `equals` utilise l'index inverse. Le
cas `range selective` reste presque stable quand la base grandit : c'est la
preuve que la plage passe par le B-Tree et ne scanne pas toutes les cles.

Le cas range a 50 % devient plus long parce qu'il doit vraiment construire et
renvoyer la moitie des entrees. Ce cout depend du nombre de resultats, pas d'un
scan cache de toute la map.

## Trajet navigateur complet

Ces chiffres incluent TypeScript, `postMessage`, le worker et le moteur WASM.

| Mesure | p50 | p95 |
|---|---:|---:|
| SET un par un | 1,20 ms | 3,50 ms |
| GET par cle | 0,30 ms | 1,00 ms |
| GET WHERE equals | 0,90 ms | 3,00 ms |
| GET WHERE range B-Tree | 1,20 ms | 2,00 ms |
| GET WHERE contains | 0,80 ms | 1,70 ms |

Pour 30 SET :

- un seul batch : 2,60 ms au total
- gain mesure par rapport a 30 messages : 15,5 fois

## Restore OPFS

Deux scenarios ont ete verifies dans un profil navigateur isole :

| Scenario | Resultat |
|---|---:|
| Snapshot avec 100 entrees | 17,80 ms |
| Lecture AOF vide apres compaction | 2,60 ms |
| Restore total snapshot | 20,40 ms |
| Replay AOF avec 1 operation | 5,10 ms |
| Restore total du test AOF | 9,90 ms |

Le test AOF a sauvegarde une cle, force le flush, recharge la page puis verifie
la presence de la cle. Le test snapshot a attendu la compaction, recharge la
page et retrouve les 100 entrees avec un AOF vide.

## React

- FPS pendant le scroll automatique : 60,0
- virtual scroll : 29 lignes DOM pour une fenetre de 520 px
- calcul teste avec 100 000 entrees : toujours 29 lignes maximum
- render granulaire : ligne modifiee `1 -> 2`, autre ligne `1 -> 1`

Le test automatique du store confirme aussi : une mise a jour de la cle `a`
notifie l'abonnement de `a`, mais pas celui de `b` ni celui de la liste.

Le dataset `demo-data/demo-100000.ndjson` contient 100 000 lignes. Il n'a pas
ete injecte completement dans ce petit PC : la garantie 100k porte ici sur le
calcul de virtualisation, tandis que le parcours navigateur a utilise 100
entrees reelles.

## Reproduire

```bash
go test -bench=. -benchmem -benchtime=300ms -count=1 ./internal/redis
cd web
npm test
npm run dev
```

Dans l'interface : `Seed 100`, puis `Benchmark`. Garder l'onglet visible pendant
la mesure des FPS, car un navigateur ralentit les onglets places en arriere-plan.
