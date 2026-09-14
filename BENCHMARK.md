# Benchmark WasmRedis

Le projet contient deux niveaux de benchmark simples.

## Dans le navigateur

Le bouton `Benchmark` de l'interface mesure directement le SDK, le Worker et le
moteur Go compile en WASM. Il affiche :

- SET et GET en p50 / p95
- GET WHERE `contains` en p50 / p95
- GET WHERE numerique via le B-Tree en p50 / p95
- temps total d'un batch
- gain du batch par rapport aux messages envoyes un par un

Le nombre d'iterations vient de `VITE_WASMREDIS_BENCHMARK_ITERATIONS`.

## Moteur Go natif

Commande reproductible :

```bash
go test -bench=. -benchmem ./internal/redis
```

Mesure du 14 septembre 2026, sous Linux amd64, Intel Core i5-12450H :

```text
BenchmarkEngineSet-8             689.9 ns/op
BenchmarkEngineGet-8              39.61 ns/op
BenchmarkEngineWhereRange-8     8466 ns/op
BenchmarkEngineBatch-8          1139 ns/op
```

Ces nombres servent de point de comparaison. Les resultats importants pour la
presentation sont ceux du bouton navigateur, car ils incluent WASM et les
messages du Worker.

## Limite actuelle

Le FPS du scroll reste a relever dans les outils de developpement du navigateur.
Le compteur orange de chaque ligne sert de preuve visuelle pour le render
granulaire : modifier une valeur doit incrementer uniquement la ligne concernee.
