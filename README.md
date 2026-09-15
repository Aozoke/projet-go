# WasmRedis

WasmRedis est une petite base de donnees `cle -> valeur`. Le moteur est ecrit
en Go, compile en WebAssembly et execute dans un Web Worker. React reste sur le
thread principal et ne touche jamais directement aux fichiers.

Le projet cherche surtout a montrer clairement le fonctionnement de Redis dans
le navigateur, sans serveur et sans architecture inutilement compliquee.

## Ce qui fonctionne

- `SET`, `GET`, `DELETE`, `ALL` et `GET WHERE`
- filtres `equals`, `contains`, `>`, `>=`, `<` et `<=`
- index inverse pour `equals` et B-Tree Go pour les plages numeriques
- TTL lazy au `GET` et balayage periodique des expirations
- batch de commandes sans interleaving cote moteur
- buffer d'ecritures dans le moteur Go
- AOF, snapshot et restore dans OPFS
- SDK TypeScript fonctionnel, generique et valide avec Zod
- React CRUD, virtual scroll maison et abonnement par ligne
- benchmarks Go et navigateur
- dataset de 100 000 entrees fourni sans chargement automatique

## Architecture

```text
React
  -> SDK TypeScript : validation + commandes
  -> Web Worker : orchestration et OPFS
  -> moteur Go/WASM
       |- state en RAM
       |- buffer d'operations
       |- index equals
       `- B-Tree numerique
            |
            `-> OPFS : aof.log + snapshot.json
```

## Comprendre le trajet d'un SET

1. React appelle `db.set("age", 25)`.
2. Le SDK valide la cle et la valeur, puis cree `SET age "n:25"`.
3. Le SDK envoie la commande au Web Worker avec `postMessage`.
4. Le parser Go transforme le texte en `Command`.
5. Le moteur met `age` dans le `state`, met a jour le B-Tree et ajoute une
   operation dans son buffer.
6. Environ une fois par seconde, le worker vide le buffer Go vers `aof.log`.
7. Environ toutes les deux minutes, il ecrit tout le state dans
   `snapshot.json`, puis vide l'AOF.

Au redemarrage, le snapshot est charge en premier, puis les operations plus
recentes de l'AOF sont rejouees. Les dates de TTL sont conservees exactement.

## Structure du depot

```text
internal/redis/       moteur, parser, TTL, buffer et B-Tree
cmd/cli/              essai manuel dans le terminal
cmd/wasm/             pont JavaScript <-> Go
web/src/sdk/          SDK TypeScript
web/src/worker/       Worker et persistance OPFS
web/src/store/        abonnements React par ligne
web/src/virtual/      calcul du virtual scroll
web/src/benchmark/    mesures navigateur et FPS
demo-data/            dataset 100 000 entrees
scripts/              build WASM et generation du dataset
presentation/         support de presentation et notes
```

## Installation et lancement

Prerequis : Go 1.22 ou plus recent et Node.js 22.

```bash
cd web
npm install
npm run dev
```

Ouvrir ensuite :

```text
http://localhost:5173/
```

Pour vider OPFS une seule fois au demarrage :

```text
http://localhost:5173/?reset=1
```

Le seed reste volontairement a 100 entrees par defaut pour les petits PC.

## Tester

Depuis la racine :

```bash
go test -race ./...
cd web
npm test
npm run build
```

Le build execute aussi `scripts/build-wasm.sh`, qui copie le runtime Go et
compile `cmd/wasm` en `web/public/wasmredis.wasm`.

## Commandes du moteur

```text
SET name "matt"
SET session "active" EX 60
GET name
DELETE name
ALL
GET WHERE key contains "demo:"
GET WHERE value equals "n:25"
GET WHERE value >= n:18
```

Le prefixe `n:` est ajoute automatiquement par le SDK aux nombres. Le CLI Go
accepte aussi les nombres simples comme `18`.

## SDK TypeScript

```ts
type Schema = { name: string; age: number };

const db = await initWasmRedis<Schema>();

await db.set("name", "matt");
await db.set("age", 25, { ex: 60 });

const result = await db
  .get()
  .where("age", ">", 18)
  .where("name", "contains", "ma")
  .exec();
```

TypeScript refuse par exemple `where("age", "contains", 18)` ou une valeur
texte pour `age`. Ce contrat est verifie pendant le build dans
`web/src/sdk/wasmRedis.typecheck.ts`.

Le stockage reste plat : `name` et `age` sont deux cles de la base, pas les
champs de plusieurs documents. Dans une chaine de filtres sur le schema, tous
les predicats doivent etre vrais pour obtenir les entrees correspondantes.

## Virtual scroll et render granulaire

La liste complete est representee par un spacer, mais React ne monte que les
lignes visibles avec une petite marge. Avec les valeurs par defaut, le test sur
100 000 entrees ne garde jamais plus de 29 lignes a afficher.

Chaque ligne s'abonne uniquement a sa cle avec `useSyncExternalStore`. Le test
automatise et le compteur orange prouvent qu'une modification notifie la ligne
cible sans re-rendre les autres lignes.

Le fichier `demo-data/demo-100000.ndjson` contient bien 100 000 entrees, mais il
n'est jamais importe au demarrage. Pour regenerer ce fichier :

```bash
node scripts/generate-demo-data.mjs 100000
```

## Configuration

Toutes les valeurs ajustables sont documentees dans `.env.example`. Copier les
variables voulues dans un fichier `.env` a la racine. Vite lit ce dossier grace
a `envDir: ".."`.

On peut regler les intervalles de flush, snapshot et balayage TTL, la taille du
buffer, le TTL par defaut, le degre du B-Tree, les fichiers OPFS, la taille du
seed, la fenetre du virtual scroll et les benchmarks.

## Benchmarks

Le rapport et la methode sont dans [BENCHMARK.md](BENCHMARK.md).

```bash
go test -bench=. -benchmem -benchtime=300ms -count=1 ./internal/redis
```

Le bouton `Benchmark` mesure le trajet complet SDK -> Worker -> WASM et le FPS
du scroll. Il affiche les latences p50/p95, le gain du batch et le restore OPFS.

## Limites assumees

- le modele est une base plate et non une base de documents
- une suppression numerique reconstruit le B-Tree pour rester simple et juste
- le bonus de synchronisation entre plusieurs onglets n'est pas implemente
- le dataset 100k est fourni, mais n'est pas charge automatiquement sur ce PC
- les chiffres du navigateur dependent de la machine et doivent etre re-mesures
