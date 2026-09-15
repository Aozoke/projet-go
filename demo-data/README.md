# Jeu de donnees

`demo-100000.ndjson` contient 100 000 paires `key/value`, une par ligne.
Il est fourni pour les tests de performance mais n'est jamais charge au
demarrage de l'application.

Pour le regenerer :

```bash
node scripts/generate-demo-data.mjs 100000
```

Sur un petit PC, garder `VITE_WASMREDIS_DEMO_ENTRY_COUNT=100`. Le fichier sert
uniquement quand on choisit volontairement de faire un test lourd.
