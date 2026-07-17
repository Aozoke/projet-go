# Roadmap WasmRedis

- moteur Go
- parser
- state en memoire
- persistance AOF
- snapshot
- restore
- TTL
- B-Tree
- WASM
- worker
- SDK TypeScript
- React
- virtual scroll
- benchmarks
- presentation


## Etat global

Projet complet : environ 3%.

On a termine le premier morceau du moteur Go : le parser simple.

## Phases

```text
Phase 0 : setup Go              fait
Phase 1.1 : parser              fait
Phase 1.2 : moteur memoire      prochaine etape
Phase 2 : persistance           pas commence
Phase 3 : batch                 pas commence
Phase 4 : filtres / index       pas commence
Phase 5 : TTL                   pas commence
WASM / React / SDK              pas commence
```

## Phase terminee : parser

Ce qui marche :

- `GET name` est parse en commande Go
- `DELETE name` est parse en commande Go
- `SET name "matt"` est parse en commande Go
- `SET message "hello world"` garde la valeur complete
- les commandes sont acceptees meme en minuscules, exemple `get name`

Erreurs gerees :

- commande inconnue : `PING name`
- cle manquante : `GET`, `DELETE`
- valeur manquante : `SET name`
- valeur sans guillemets : `SET name matt`
- commande vide : `   `

Pourquoi :

Le parser est la porte d'entree du moteur. Si une commande est inconnue ou mal formee, on veut une erreur claire au lieu de laisser le moteur faire n'importe quoi.

## Phase suivante : moteur memoire

But :

Creer une structure Go qui garde les donnees en RAM avec une map.

Version simple visee :

```text
SET name "matt"  -> stocke name = matt
GET name         -> renvoie matt
DELETE name      -> supprime name
```

Ce qu'on ne fait pas encore :

- persistance fichier
- AOF
- snapshot
- TTL
- filtres
- WASM
- React
