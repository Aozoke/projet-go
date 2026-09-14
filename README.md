# WasmRedis

WasmRedis est une petite base cle-valeur ecrite en Go et executee dans le
navigateur grace a WebAssembly.

Le projet reste volontairement simple : le but est de comprendre le trajet
d'une commande, de React jusqu'au moteur Go, puis jusqu'a la persistance OPFS.

## Ce qui fonctionne

- moteur cle-valeur en Go
- commandes `SET`, `GET`, `DELETE`, `ALL` et `GET WHERE`
- TTL avec expiration a la lecture et balayage periodique
- index inverse pour `equals`
- B-Tree pour les filtres numeriques `>`, `>=`, `<` et `<=`
- batch de commandes
- compilation Go vers WebAssembly
- Web Worker pour ne pas bloquer React
- AOF et snapshot dans OPFS
- SDK TypeScript avec validation Zod
- query builder fonctionnel `get().where().exec()`
- UI React CRUD avec filtre et seed
- virtual scroll code a la main
- store externe par ligne pour le render granulaire
- benchmark Go et benchmark depuis le navigateur

## Architecture

```text
React
  -> SDK TypeScript
  -> Web Worker
  -> moteur Go compile en WASM
  -> state en RAM + index
  -> buffer AOF
  -> OPFS : aof.log + snapshot.json
```

## Structure

```text
internal/redis/       moteur, parser, TTL et B-Tree
cmd/cli/              test manuel dans le terminal
cmd/wasm/             pont entre Go et JavaScript
scripts/build-wasm.sh build du fichier WASM
web/src/sdk/          SDK TypeScript
web/src/worker/       Worker, buffer et OPFS
web/src/store/        abonnements React par ligne
web/src/benchmark/    mesures dans le navigateur
web/src/App.tsx       interface React
```

## Tester le moteur Go

```bash
go test ./...
```

Lancer le petit terminal :

```bash
go run ./cmd/cli
```

Commandes utiles :

```text
SET name "matt"
SET session "active" EX 60
GET name
DELETE name
ALL
GET WHERE key contains "demo:"
GET WHERE value equals "25"
GET WHERE value >= 18
```

Notre base est plate : une entree contient une `key` et une `value`. Pour cette
version, `GET WHERE` accepte donc ces deux noms de champ. Les comparaisons de
plage fonctionnent uniquement sur une `value` numerique.

## Lancer l'interface

Dans WSL :

```bash
cd web
npm install
npm run dev
```

Puis ouvrir :

```text
http://localhost:5173
```

Si le terminal choisit le npm de Windows, recharger la configuration WSL :

```bash
source ~/.bashrc
```

L'interface permet de :

- ajouter ou modifier une entree
- donner un TTL optionnel en secondes
- supprimer une entree
- filtrer par cle ou valeur
- ajouter 100 valeurs numeriques avec `Seed 100`
- forcer le flush AOF
- vider toute la base
- lancer un petit benchmark

Cette URL vide OPFS avant le demarrage :

```text
http://localhost:5173/?reset=1
```

## SDK TypeScript

```ts
type Schema = Record<string, string>;

const db = await initWasmRedis<Schema>();

await db.set("session", "active", { ex: 60 });

const adults = await db
  .get()
  .where("value", ">=", 18)
  .exec();
```

Plusieurs `where` peuvent etre chaines. Le SDK execute les filtres puis garde
les cles presentes dans tous les resultats.

## TTL

`SET session "active" EX 60` calcule une date d'expiration dans le moteur Go.

- expiration lazy : `GET` supprime une cle deja expiree
- expiration active : le Worker demande regulierement a Go de balayer les cles
- la suppression est ajoutee a l'AOF
- le snapshot conserve la date d'expiration

## B-Tree

Les valeurs numeriques sont ajoutees dans un B-Tree Go. Une commande comme
`GET WHERE value >= 18` lit cet index au lieu de parcourir la map principale.

Pour garder l'implementation lisible, le B-Tree et l'index `equals` sont
reconstruits apres chaque ecriture. C'est correct pour la petite demo ; une
version tres volumineuse devrait mettre les index a jour element par element.

## Render granulaire

Le virtual scroll ne monte que les lignes visibles. En plus, chaque ligne
s'abonne a sa propre cle avec `useSyncExternalStore`. Modifier une valeur ne
notifie donc que sa ligne. Le compteur orange visible dans la ligne permet de
le verifier.

## Configuration

Toutes les valeurs ajustables sont documentees dans `.env.example`. Pour les
surcharger, creer un fichier `.env` a la racine du projet.

On peut notamment regler les intervalles de flush, snapshot et balayage TTL,
le TTL par defaut, le degre du B-Tree, le seed, le virtual scroll et le nombre
d'iterations du benchmark.

## Benchmarks

Voir [BENCHMARK.md](BENCHMARK.md).

```bash
go test -bench=. -benchmem ./internal/redis
```

Le bouton `Benchmark` mesure aussi le vrai trajet SDK -> Worker -> WASM dans le
navigateur et affiche les p50 / p95.

## Limites connues

- le modele reste une base plate `key -> value`, pas un stockage de documents
- la suppression B-Tree est remplacee par une reconstruction simple
- le test a un million d'entrees n'est pas adapte au petit PC de developpement
- le FPS du scroll et le temps de restore restent a relever manuellement
- la presentation finale reste a terminer
