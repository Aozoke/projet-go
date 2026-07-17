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

Projet complet : environ 2%.

On est au tout debut du moteur Go.

## Phases

```text
Phase 0 : setup Go              fait
Phase 1.1 : parser              en cours
Phase 1.2 : moteur memoire      pas commence
Phase 2 : persistance           pas commence
Phase 3 : batch                 pas commence
Phase 4 : filtres / index       pas commence
Phase 5 : TTL                   pas commence
WASM / React / SDK              pas commence
```

## Phase en cours : parser

Ce qui marche deja :

- `GET name` est parse en commande Go
- `GET` renvoie une erreur propre si la cle manque

Prochaine mini etape :

- refuser une commande inconnue comme `PING name`

Pourquoi :

Le parser est la porte d'entree du moteur. Si une commande est inconnue ou mal formee, on veut une erreur claire au lieu de laisser le moteur faire n'importe quoi.


