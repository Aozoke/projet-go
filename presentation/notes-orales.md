# Notes orales - WasmRedis

Support de 10 slides pour environ 15 minutes.

## 1. WasmRedis
Je presente une base cle-valeur volontairement simple. Le coeur metier est en Go, compile en WebAssembly. React pilote le moteur sans serveur, et OPFS garde les donnees.

## 2. Une commande, cinq etapes
Je pars d'un clic dans React. Le SDK valide puis envoie un message au worker. Le worker appelle Go/WASM. Go modifie le state et son buffer, puis le worker vide ce buffer dans OPFS. C'est la carte mentale a garder pour la suite.

- React ne bloque jamais : le moteur et le disque vivent dans le worker.
- La RAM donne la vitesse ; AOF + snapshot donnent la persistance.

## 3. Le parser protege l'entree
Le parser recoit une string. Le switch choisit la seule fonction capable de parser cette commande. Dans le cas GET, je regarde si le mot suivant est WHERE. Les erreurs sont renvoyees proprement, donc aucune entree brute n'arrive dans le state.

Code affiche :
- `internal/redis/parser.go:44-51`

- GET WHERE est distingue d'un GET par cle.
- Toute commande inconnue est rejetee avant le moteur.

## 4. Le buffer appartient au moteur Go
Chaque SET, DELETE ou expiration ajoute une Operation dans le buffer Go. Le worker appelle DrainBuffer environ chaque seconde. La copie est renvoyee au worker et le slice est reutilise, ce qui garde le mecanisme simple.

Code affiche :
- `internal/redis/engine.go:230-239`

- Le mutex interdit deux vidages concurrents.
- DrainBuffer copie les operations puis remet la file a zero.

## 5. AOF puis snapshot
Le worker recupere le buffer Go et le transforme en lignes JSON. appendOpfsText ouvre un SyncAccessHandle exclusif. Toutes les taches disque passent aussi par storageLock. Toutes les deux minutes, un snapshot complet est ecrit puis l'AOF est vide.

Code affiche :
- `web/src/worker/wasmRedis.worker.ts:386-400`

- L'AOF est ajoute en fin de fichier, jamais reecrit a chaque SET.
- En cas d'echec OPFS, les operations retournent dans retryAof.

## 6. Les plages passent par le B-Tree
Pour une recherche inferieure, le B-Tree est parcouru dans l'ordre croissant. Des qu'une valeur ne correspond plus, la fonction retourne false et coupe le parcours. Le cas superieur fait la meme chose en sens inverse.

Code affiche :
- `internal/redis/btree.go:95-103`

- equals utilise un index inverse ; contains assume un scan.
- Une plage selective reste a environ 0,3 us jusqu'a 10 000 entrees.

## 7. Le TTL supprime vraiment la cle
Une valeur garde une date ExpiresAt. Si elle est depassee, cette fonction retire la cle du state et des deux index. L'appelant cree ensuite une operation DELETE pour l'AOF. Une horloge injectable rend ce comportement testable sans attendre.

Code affiche :
- `internal/redis/engine.go:451-459`

- Expiration lazy au GET, plus balayage actif periodique.
- Le restore garde ExpiresAt et ne remplit pas de nouveau le buffer.

## 8. Le SDK rend les requetes typees
Le generique Schema relie le champ, l'operateur et la valeur. TypeScript refuse par exemple age contains ou age superieur a une string. Le SDK est une factory de fonctions, sans classe ni new.

Code affiche :
- `web/src/sdk/wasmRedis.ts:35-42`

- age: number autorise les plages ; name: string autorise contains.
- Zod revalide aussi les donnees au runtime avant postMessage.

## 9. 100 000 entrees, 29 lignes DOM
Le calcul transforme scrollTop en startIndex et endIndex, avec huit lignes de marge. Le test utilise 100 000 entrees et ne depasse jamais 29 lignes. Chaque EntryRow s'abonne a une seule cle avec useSyncExternalStore, donc une edition ne reveille pas les autres.

Code affiche :
- `web/src/virtual/getVirtualRange.ts:22-27`
- `web/src/store/entryStore.ts:60-64`

- Le spacer simule la hauteur totale ; seule la tranche visible est montee.
- Modifier une ligne : compteur 1 -> 2 ; autre ligne : 1 -> 1.

## 10. Ce que la demo prouve
Je termine par une demo courte : ajouter une cle avec TTL, forcer le flush, recharger, lancer Seed 100, filtrer value >= 50, editer une ligne puis lancer Benchmark. Le dataset 100k existe dans le repo mais n'est pas charge par defaut sur ce petit PC.
