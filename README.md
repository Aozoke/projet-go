# WasmRedis

WasmRedis est une base cle-valeur inspiree de Redis.

Le projet final devra tourner dans le navigateur avec un moteur ecrit en Go,
compile en WebAssembly, persiste via OPFS, pilote par un SDK TypeScript et
visualise dans une UI React.

Pour avancer sans se perdre, on construit d'abord le moteur Go en natif avec des tests. 
Le portage WASM, le worker, le SDK et React viendront seulement apres.

## Etape actuelle

Phase 1.1 : parser simple termine.

Ce qui existe maintenant :

- un module Go : `github.com/Aozoke/projet-go`
- un point d'entree minimal : `main.go`
- la dependance `github.com/samber/lo`
- un fichier `.env.example` qui liste les constantes configurables prevues
- un parser de commandes texte pour `GET`, `DELETE` et `SET`
- des tests pour verifier le parser et les erreurs simples

## Ce que le parser sait faire

Commandes valides :

```text
GET name
DELETE name
SET name "matt"
SET message "hello world"
```

Erreurs gerees :

- commande inconnue, exemple `PING name`
- cle manquante, exemple `GET` ou `DELETE`
- valeur manquante, exemple `SET name`
- valeur de `SET` sans guillemets, exemple `SET name matt`

## Roadmap courte

1. Parser les commandes texte : `SET`, `GET`, `DELETE`.
2. Creer le moteur en memoire : une map `cle -> valeur`.
3. Brancher parser + moteur avec des tests.
4. Ajouter la persistance : AOF, snapshot, restore.
5. Ajouter les batches.
6. Ajouter les filtres et les index.
7. Ajouter le TTL.
8. Compiler en WASM et deplacer le moteur dans un Web Worker.
9. Creer le SDK TypeScript.
10. Creer l'UI React avec virtual scroll.

## Prochaine etape

Creer le moteur en memoire.

Objectif simple :

```text
SET name "matt"  -> ecrit name = matt
GET name         -> lit matt
DELETE name      -> supprime name
```

## Commandes utiles

```bash
go run .
go test ./...
```

## Vocabulaire

- `state` : la base vivante en RAM.
- `buffer` : la liste des ecritures qui attendent d'etre sauvegardees.
- `AOF` : le journal persistant des ecritures.
- `snapshot` : une photo complete de la base a un instant donne.
- `restore` : reconstruction du `state` depuis le snapshot puis l'AOF.
