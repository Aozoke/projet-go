package redis

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Engine garde la base en RAM et ses deux index pour accelerer les recherches.
// Le state et les index sont modifies sur place pour ne pas recopier toute la base a chaque SET.
type Engine struct {
	// mu protege les donnees/index, batchMu les lots, bufferMu la file d'ecritures.
	mu                sync.RWMutex
	batchMu           sync.Mutex
	bufferMu          sync.Mutex
	state             Snapshot                       // Les donnees actuelles : cle -> valeur avec expiration.
	buffer            []Operation                    // Les ecritures en attente de sauvegarde, dans l'ordre.
	equalsIndex       map[string]map[string]struct{} // Valeur -> ensemble des cles qui la portent.
	numberIndex       *BTree                         // Les valeurs numeriques rangees dans l'ordre.
	btreeDegree       int
	defaultTTLSeconds int64
	now               func() time.Time // Donne l'heure reelle, ou l'heure simulee dans un test.
}

// NewEngine cree un moteur vide avec les reglages par defaut.
func NewEngine() *Engine {
	return NewEngineWithConfig(defaultConfig())
}

// NewEngineWithConfig prepare la base, les index et l'horloge avec les reglages fournis.
func NewEngineWithConfig(config Config) *Engine {
	// On complete le degre invalide et l'horloge absente avec les valeurs de secours.
	defaults := defaultConfig()
	if config.BTreeDegree < 2 {
		config.BTreeDegree = defaults.BTreeDegree
	}
	if config.Now == nil {
		config.Now = defaults.Now
	}

	return &Engine{
		state:             Snapshot{},
		buffer:            []Operation{},
		equalsIndex:       map[string]map[string]struct{}{},
		numberIndex:       NewBTree(config.BTreeDegree),
		btreeDegree:       config.BTreeDegree,
		defaultTTLSeconds: config.DefaultTTLSeconds,
		now:               config.Now,
	}
}

// Configure applique la configuration reçue du worker au moteur WASM.
func (engine *Engine) Configure(defaultTTLSeconds int64, btreeDegree int) {
	engine.mu.Lock()
	defer engine.mu.Unlock()

	engine.defaultTTLSeconds = defaultTTLSeconds
	if btreeDegree >= 2 {
		engine.btreeDegree = btreeDegree
	}
	// On refait les index, notamment pour appliquer le nouveau degre du B-Tree.
	engine.rebuildIndexesLocked()
}

// Set ajoute ou remplace une valeur en RAM en utilisant le TTL par defaut.
// Cette methode seule ne remplit pas le buffer : Execute s'en charge pour les commandes.
func (engine *Engine) Set(key string, value string) {
	engine.SetWithTTL(key, value, 0)
}

// SetWithTTL ecrit la valeur et sa date d'expiration, puis met les index a jour.
// Il renvoie aussi la valeur stockee, pour que execute puisse preparer l'operation AOF.
func (engine *Engine) SetWithTTL(key string, value string, ttlSeconds int64) StoredValue {
	engine.mu.Lock()
	defer engine.mu.Unlock()

	if ttlSeconds == 0 {
		ttlSeconds = engine.defaultTTLSeconds
	}

	stored := StoredValue{Value: value}
	if ttlSeconds > 0 {
		// On convertit la duree en date : maintenant + TTL, exprime en millisecondes.
		stored.ExpiresAt = engine.now().Add(time.Duration(ttlSeconds) * time.Second).UnixMilli()
	}

	// On garde l'ancienne valeur pour retirer son ancienne place dans les index.
	previous, hadPrevious := engine.state[key]
	engine.state[key] = stored
	engine.updateIndexesLocked(key, previous, hadPrevious, stored)
	return stored
}

// Get lit une cle en RAM et renvoie une erreur si elle est absente ou expiree.
// Cet appel direct n'ajoute pas de suppression au buffer en cas d'expiration.
func (engine *Engine) Get(key string) (string, error) {
	value, _, err := engine.getAndExpire(key)
	return value, err
}

// getAndExpire lit la valeur, mais supprime aussi une cle dont le TTL est termine.
// Le bool vaut true si une expiration vient d'etre supprimee : execute preparera un DELETE.
func (engine *Engine) getAndExpire(key string) (string, bool, error) {
	engine.mu.Lock()
	defer engine.mu.Unlock()

	stored, ok := engine.state[key]
	if !ok {
		return "", false, fmt.Errorf("key not found")
	}

	if engine.isExpired(stored) {
		// Expiration a la lecture : la cle disparait du state et des deux index.
		delete(engine.state, key)
		engine.removeFromEqualsIndexLocked(key, stored.Value)
		engine.removeFromNumberIndexLocked(stored.Value)
		return "", true, fmt.Errorf("key not found")
	}

	return stored.Value, false, nil
}

// Delete retire la cle de la RAM et des index. Une cle deja absente ne pose pas d'erreur.
// Comme Set, cette methode seule ne note pas d'operation dans le buffer.
func (engine *Engine) Delete(key string) {
	engine.mu.Lock()
	defer engine.mu.Unlock()

	stored, exists := engine.state[key]
	delete(engine.state, key)
	if exists {
		engine.removeFromEqualsIndexLocked(key, stored.Value)
		engine.removeFromNumberIndexLocked(stored.Value)
	}
}

// Entries renvoie les entrees encore valides, triees par cle.
// Cet appel direct ignore les operations d'expiration ; ALL les recupere via execute.
func (engine *Engine) Entries() []Entry {
	entries, _ := engine.entriesAndExpiredWrites()
	return entries
}

// entriesAndExpiredWrites prepare la liste des entrees et les suppressions dues au TTL.
func (engine *Engine) entriesAndExpiredWrites() ([]Entry, []Operation) {
	engine.mu.Lock()
	defer engine.mu.Unlock()

	writes := engine.removeExpiredLocked()
	entries := make([]Entry, 0, len(engine.state))
	for key, stored := range engine.state {
		entries = append(entries, Entry{Key: key, Value: stored.Value})
	}

	// Une map n'a pas d'ordre garanti : on trie pour obtenir une liste stable.
	sortEntries(entries)
	return entries, writes
}

// Snapshot fait une copie des donnees valides, avec leurs dates d'expiration.
// Il ne cree pas de fichier : le code de persistance enregistrera cette copie.
func (engine *Engine) Snapshot() Snapshot {
	engine.mu.Lock()
	defer engine.mu.Unlock()

	engine.removeExpiredLocked()
	// La copie evite de donner la map vivante a l'appelant.
	snapshot := make(Snapshot, len(engine.state))
	for key, stored := range engine.state {
		snapshot[key] = stored
	}

	return snapshot
}

// LoadSnapshot remplace la RAM avec un snapshot deja lu, sans relancer les TTL.
// Il ignore les cles deja expirees et reconstruit les index a partir des donnees restantes.
func (engine *Engine) LoadSnapshot(snapshot Snapshot) {
	engine.mu.Lock()
	defer engine.mu.Unlock()

	engine.state = make(Snapshot, len(snapshot))
	for key, stored := range snapshot {
		if !engine.isExpired(stored) {
			engine.state[key] = stored
		}
	}
	engine.rebuildIndexesLocked()
}

// ExecuteText est le passage du texte au moteur : parser, puis executer la commande.
// Une erreur de parsing renvoie un resultat d'echec sans executer d'action.
func (engine *Engine) ExecuteText(input string) (Result, []Operation) {
	command, err := ParseCommand(input)
	if err != nil {
		return Result{OK: false, Error: err.Error()}, nil
	}

	return engine.Execute(command)
}

// Execute lance une commande deja structuree et ajoute ses ecritures au buffer.
// La reponse OK ne signifie pas encore que ces ecritures ont ete sauvegardees sur disque.
func (engine *Engine) Execute(command Command) (Result, []Operation) {
	result, writes := engine.execute(command)
	engine.appendToBuffer(writes)
	return result, writes
}

// execute choisit l'action avec switch et renvoie la reponse plus les ecritures produites.
// Execute ajoutera ensuite les ecritures au buffer.
func (engine *Engine) execute(command Command) (Result, []Operation) {
	switch command.Type {
	case CommandSet:
		stored := engine.SetWithTTL(command.Key, command.Value, command.TTLSeconds)
		// Le journal garde la date exacte d'expiration pour ne pas prolonger le TTL au restore.
		operation := Operation{
			Type:      CommandSet,
			Key:       command.Key,
			Value:     command.Value,
			ExpiresAt: stored.ExpiresAt,
		}
		return Result{OK: true}, []Operation{operation}
	case CommandGet:
		value, expired, err := engine.getAndExpire(command.Key)
		if err != nil {
			if expired {
				// Meme une lecture peut produire un DELETE si elle decouvre une expiration.
				return Result{OK: false, Error: err.Error()}, []Operation{{Type: CommandDelete, Key: command.Key}}
			}
			return Result{OK: false, Error: err.Error()}, nil
		}

		return Result{OK: true, Value: value}, nil
	case CommandWhere:
		// Le filtre peut aussi retirer des cles expirees : on garde leurs ecritures.
		entries, writes, err := engine.Query(command.FilterField, command.Operator, command.FilterValue)
		if err != nil {
			return Result{OK: false, Error: err.Error()}, writes
		}
		return Result{OK: true, Entries: entries}, writes
	case CommandDelete:
		engine.Delete(command.Key)
		return Result{OK: true}, []Operation{{Type: CommandDelete, Key: command.Key}}
	case CommandAll:
		entries, writes := engine.entriesAndExpiredWrites()
		return Result{OK: true, Entries: entries}, writes
	default:
		return Result{OK: false, Error: "unknown command"}, nil
	}
}

// appendToBuffer ajoute les operations en attente sans acceder au disque.
// La liste est modifiee sur place pour eviter de recopier tout le buffer a chaque commande.
func (engine *Engine) appendToBuffer(writes []Operation) {
	if len(writes) == 0 {
		return
	}

	engine.bufferMu.Lock()
	defer engine.bufferMu.Unlock()
	engine.buffer = append(engine.buffer, writes...)
}

// DrainBuffer rend les écritures au worker puis remet la file à zéro.
// Ce n'est pas encore une sauvegarde : l'appelant doit ecrire ces operations dans l'AOF.
func (engine *Engine) DrainBuffer() []Operation {
	engine.bufferMu.Lock()
	defer engine.bufferMu.Unlock()

	// On rend une copie pour que les prochaines ecritures n'ecrasent pas celles a sauvegarder.
	writes := make([]Operation, len(engine.buffer))
	copy(writes, engine.buffer)
	// On vide la file en conservant la memoire pour les prochaines ecritures.
	engine.buffer = engine.buffer[:0]
	return writes
}

// BufferSize compte les operations en attente, sous le meme verrou que les ajouts/retraits.
func (engine *Engine) BufferSize() int {
	engine.bufferMu.Lock()
	defer engine.bufferMu.Unlock()
	return len(engine.buffer)
}

// ReplayOperations rejoue l'AOF sans remettre les opérations dans le buffer.
// Il recoit une liste deja lue par la persistance ; il n'ouvre aucun fichier ici.
func (engine *Engine) ReplayOperations(operations []Operation) error {
	engine.mu.Lock()
	defer engine.mu.Unlock()

	// L'ordre compte : un DELETE apres un SET doit bien enlever la valeur restauree.
	for _, operation := range operations {
		if operation.Key == "" {
			return fmt.Errorf("operation key is missing")
		}

		switch operation.Type {
		case CommandSet:
			stored := StoredValue{Value: operation.Value, ExpiresAt: operation.ExpiresAt}
			if engine.isExpired(stored) {
				delete(engine.state, operation.Key)
				continue
			}
			engine.state[operation.Key] = stored
		case CommandDelete:
			delete(engine.state, operation.Key)
		default:
			return fmt.Errorf("unsupported operation: %s", operation.Type)
		}
	}

	// Les index sont reconstruits une seule fois, apres toutes les operations valides.
	engine.rebuildIndexesLocked()
	return nil
}

// Query choisit comment chercher : index pour equals sur valeur, parcours pour contains,
// B-Tree pour les comparaisons numeriques. Il renvoie entrees, suppressions TTL et erreur.
func (engine *Engine) Query(field FilterField, operator FilterOperator, expected string) ([]Entry, []Operation, error) {
	engine.mu.Lock()
	defer engine.mu.Unlock()

	writes := make([]Operation, 0)
	// Un champ comme age designe ici la cle "age", pas un champ dans un objet JSON.
	if field != FilterKey && field != FilterValue {
		if stored, ok := engine.state[string(field)]; ok && engine.removeIfExpiredLocked(string(field), stored) {
			writes = append(writes, Operation{Type: CommandDelete, Key: string(field)})
			return nil, writes, nil
		}
		entries, err := engine.querySchemaFieldLocked(string(field), operator, expected)
		return entries, writes, err
	}

	if operator == OperatorEquals {
		entries, expiredWrites := engine.queryEqualsLocked(field, expected)
		writes = append(writes, expiredWrites...)
		sortEntries(entries)
		return entries, writes, nil
	}

	if operator == OperatorContains {
		entries, expiredWrites := engine.queryContainsLocked(field, expected)
		writes = append(writes, expiredWrites...)
		sortEntries(entries)
		return entries, writes, nil
	}

	if field != FilterValue {
		return nil, writes, fmt.Errorf("range filters only work on value")
	}

	// Une plage compare des nombres : on transforme par exemple "18" ou "n:18" en 18.
	target, err := numberValue(expected)
	if err != nil {
		return nil, writes, fmt.Errorf("range filter value must be a number")
	}

	// Le B-Tree fournit les candidats, sans parcourir toute la map state.
	items := engine.numberIndex.RangeItems(operator, target)
	entries := make([]Entry, 0, len(items))
	// seen evite de renvoyer plusieurs fois la meme cle.
	seen := map[string]struct{}{}
	for _, item := range items {
		stored, ok := engine.state[item.Key]
		// On ecarte un candidat absent ou qui ne correspond plus a la valeur actuelle.
		if !ok || !isCurrentNumberItem(stored.Value, item) {
			continue
		}
		if engine.removeIfExpiredLocked(item.Key, stored) {
			writes = append(writes, Operation{Type: CommandDelete, Key: item.Key})
			continue
		}
		if _, duplicate := seen[item.Key]; duplicate {
			continue
		}
		seen[item.Key] = struct{}{}
		entries = append(entries, Entry{Key: item.Key, Value: stored.Value})
	}

	sortEntries(entries)
	return entries, writes, nil
}

// queryEqualsLocked cherche une egalite exacte : cle dans state, ou valeur dans equalsIndex.
// Le suffixe Locked indique que l'appelant doit deja tenir le verrou engine.mu.
// Cette regle vaut pour toutes les methodes dont le nom se termine par Locked.
func (engine *Engine) queryEqualsLocked(field FilterField, expected string) ([]Entry, []Operation) {
	if field == FilterKey {
		stored, ok := engine.state[expected]
		if !ok {
			return nil, nil
		}
		if engine.removeIfExpiredLocked(expected, stored) {
			return nil, []Operation{{Type: CommandDelete, Key: expected}}
		}
		return []Entry{{Key: expected, Value: stored.Value}}, nil
	}

	// L'index donne seulement les cles portant cette valeur, pas toutes celles de la base.
	keys := engine.equalsIndex[expected]
	entries := make([]Entry, 0, len(keys))
	writes := make([]Operation, 0)
	for key := range keys {
		stored, ok := engine.state[key]
		if !ok {
			delete(keys, key)
			continue
		}
		if engine.removeIfExpiredLocked(key, stored) {
			writes = append(writes, Operation{Type: CommandDelete, Key: key})
			continue
		}
		entries = append(entries, Entry{Key: key, Value: stored.Value})
	}
	return entries, writes
}

// queryContainsLocked parcourt les entrees pour chercher un morceau de texte.
// Il n'y a pas d'index pour contains : strings.Contains teste chaque cle ou valeur.
func (engine *Engine) queryContainsLocked(field FilterField, expected string) ([]Entry, []Operation) {
	expected = comparisonText(expected)
	entries := make([]Entry, 0)
	writes := make([]Operation, 0)
	for key, stored := range engine.state {
		if engine.removeIfExpiredLocked(key, stored) {
			writes = append(writes, Operation{Type: CommandDelete, Key: key})
			continue
		}
		text := comparisonText(stored.Value)
		if field == FilterKey {
			text = key
		}
		if strings.Contains(text, expected) {
			entries = append(entries, Entry{Key: key, Value: stored.Value})
		}
	}
	return entries, writes
}

// querySchemaFieldLocked filtre une cle du schema, par exemple age > 18.
// Il peut donc renvoyer au plus une entree ; les plages passent aussi par le B-Tree.
func (engine *Engine) querySchemaFieldLocked(key string, operator FilterOperator, expected string) ([]Entry, error) {
	stored, ok := engine.state[key]
	if !ok {
		return nil, nil
	}

	if operator == OperatorEquals {
		if stored.Value == expected {
			return []Entry{{Key: key, Value: stored.Value}}, nil
		}
		return nil, nil
	}

	if operator == OperatorContains {
		if strings.Contains(comparisonText(stored.Value), comparisonText(expected)) {
			return []Entry{{Key: key, Value: stored.Value}}, nil
		}
		return nil, nil
	}

	target, err := numberValue(expected)
	if err != nil {
		return nil, fmt.Errorf("range filter value must be a number")
	}

	for _, item := range engine.numberIndex.RangeItems(operator, target) {
		if item.Key == key && isCurrentNumberItem(stored.Value, item) {
			return []Entry{{Key: key, Value: stored.Value}}, nil
		}
	}
	return nil, nil
}

// SweepExpired supprime les cles expirees et ajoute leurs DELETE au buffer.
// Le worker l'appelle periodiquement : cette methode ne lance pas de minuterie elle-meme.
func (engine *Engine) SweepExpired() []Operation {
	engine.mu.Lock()
	writes := engine.removeExpiredLocked()
	engine.mu.Unlock()
	engine.appendToBuffer(writes)
	return writes
}

// removeExpiredLocked parcourt la base, supprime les expirations et prepare leurs DELETE.
// Il ne remplit pas lui-meme le buffer : son appelant recupere la liste d'operations.
func (engine *Engine) removeExpiredLocked() []Operation {
	writes := make([]Operation, 0)
	// Ce bool evite de reconstruire le B-Tree a chaque suppression pendant le balayage.
	numberIndexChanged := false
	for key, stored := range engine.state {
		if engine.isExpired(stored) {
			delete(engine.state, key)
			engine.removeFromEqualsIndexLocked(key, stored.Value)
			if _, err := numberValue(stored.Value); err == nil {
				numberIndexChanged = true
			}
			writes = append(writes, Operation{Type: CommandDelete, Key: key})
		}
	}
	if numberIndexChanged {
		engine.rebuildNumberIndexLocked()
	}
	return writes
}

// isExpired repond true si une date d'expiration existe ET si cette date est atteinte.
// Une date a zero signifie que la cle n'expire pas.
func (engine *Engine) isExpired(stored StoredValue) bool {
	return stored.ExpiresAt > 0 && stored.ExpiresAt <= engine.now().UnixMilli()
}

// removeIfExpiredLocked retire une cle expiree du state et des index.
// true indique a l'appelant qu'il faut aussi preparer une operation DELETE.
func (engine *Engine) removeIfExpiredLocked(key string, stored StoredValue) bool {
	if !engine.isExpired(stored) {
		return false
	}

	delete(engine.state, key)
	engine.removeFromEqualsIndexLocked(key, stored.Value)
	engine.removeFromNumberIndexLocked(stored.Value)
	return true
}

// rebuildIndexesLocked reconstruit les deux index depuis le state, notamment au restore.
// Les valeurs texte vont dans equalsIndex ; les valeurs convertibles en nombres vont aussi au B-Tree.
func (engine *Engine) rebuildIndexesLocked() {
	engine.equalsIndex = map[string]map[string]struct{}{}
	engine.numberIndex = NewBTree(engine.btreeDegree)

	for key, stored := range engine.state {
		keys := engine.equalsIndex[stored.Value]
		// Plusieurs cles peuvent avoir la meme valeur : on les regroupe dans une map.
		if keys == nil {
			keys = map[string]struct{}{}
			engine.equalsIndex[stored.Value] = keys
		}
		keys[key] = struct{}{}

		value, err := numberValue(stored.Value)
		if err == nil {
			engine.numberIndex.Insert(BTreeItem{Value: value, Key: key})
		}
	}
}

// updateIndexesLocked retire l'ancienne association et ajoute la nouvelle apres un SET.
// Si l'ancienne valeur etait numerique, cette version simple reconstruit tout le B-Tree.
func (engine *Engine) updateIndexesLocked(key string, previous StoredValue, hadPrevious bool, stored StoredValue) {
	if hadPrevious {
		engine.removeFromEqualsIndexLocked(key, previous.Value)
	}

	keys := engine.equalsIndex[stored.Value]
	if keys == nil {
		keys = map[string]struct{}{}
		engine.equalsIndex[stored.Value] = keys
	}
	keys[key] = struct{}{}

	if hadPrevious {
		if _, err := numberValue(previous.Value); err == nil {
			// Le state contient déjà la nouvelle valeur : une reconstruction retire
			// l'ancienne valeur numérique et ajoute la nouvelle une seule fois.
			engine.rebuildNumberIndexLocked()
			return
		}
	}

	if value, err := numberValue(stored.Value); err == nil {
		engine.numberIndex.Insert(BTreeItem{Value: value, Key: key})
	}
}

// removeFromEqualsIndexLocked retire une cle du groupe des cles portant cette valeur.
// Quand le groupe devient vide, on retire aussi la valeur de l'index.
func (engine *Engine) removeFromEqualsIndexLocked(key string, value string) {
	keys := engine.equalsIndex[value]
	delete(keys, key)
	if len(keys) == 0 {
		delete(engine.equalsIndex, value)
	}
}

// removeFromNumberIndexLocked reconstruit le B-Tree si la valeur retiree etait numerique.
// Le state doit deja etre a jour. C'est simple, mais couteux sur une grosse base.
func (engine *Engine) removeFromNumberIndexLocked(value string) {
	if _, err := numberValue(value); err == nil {
		engine.rebuildNumberIndexLocked()
	}
}

// rebuildNumberIndexLocked repart d'un B-Tree vide et y remet les nombres du state.
func (engine *Engine) rebuildNumberIndexLocked() {
	engine.numberIndex = NewBTree(engine.btreeDegree)
	for key, stored := range engine.state {
		if value, err := numberValue(stored.Value); err == nil {
			engine.numberIndex.Insert(BTreeItem{Value: value, Key: key})
		}
	}
}

// isCurrentNumberItem verifie que le nombre indexe correspond encore a la valeur en RAM.
func isCurrentNumberItem(value string, item BTreeItem) bool {
	current, err := numberValue(value)
	return err == nil && current == item.Value
}

// numberValue convertit du texte en nombre decimal, avec ou sans le prefixe SDK "n:".
// ParseFloat renvoie une erreur si le texte n'est pas un nombre.
func numberValue(value string) (float64, error) {
	if strings.HasPrefix(value, "n:") {
		value = strings.TrimPrefix(value, "n:")
	}
	return strconv.ParseFloat(value, 64)
}

// comparisonText retire le prefixe de type du SDK avant une recherche contains.
// s: texte, n: nombre, b: booleen, j: JSON. Sans prefixe reconnu, le texte reste identique.
func comparisonText(value string) string {
	for _, prefix := range []string{"s:", "n:", "b:", "j:"} {
		if strings.HasPrefix(value, prefix) {
			return strings.TrimPrefix(value, prefix)
		}
	}
	return value
}

// ExecuteBatch execute les commandes dans l'ordre et garde une reponse par commande.
// Une erreur n'annule pas les ecritures precedentes et n'empeche pas de traiter la suite.
func (engine *Engine) ExecuteBatch(commands []string) BatchResult {
	// Ce verrou fait attendre les autres appels a ExecuteBatch jusqu'a la fin du lot.
	// Il ne bloque pas a lui seul un appel direct a Set ou ExecuteText.
	engine.batchMu.Lock()
	defer engine.batchMu.Unlock()

	results := make([]Result, 0, len(commands))
	writes := make([]Operation, 0)

	for _, commandText := range commands {
		result, commandWrites := engine.ExecuteText(commandText)
		// Chaque resultat reste a la meme position que sa commande ; les ecritures sont regroupees.
		results = append(results, result)
		writes = append(writes, commandWrites...)
	}

	return BatchResult{Results: results, Writes: writes, BufferSize: engine.BufferSize()}
}

// sortEntries trie la liste recue sur place, par ordre de texte des cles.
func sortEntries(entries []Entry) {
	sort.Slice(entries, func(left int, right int) bool {
		return entries[left].Key < entries[right].Key
	})
}
