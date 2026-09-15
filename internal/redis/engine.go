package redis

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Engine garde la base en RAM et ses deux index de recherche.
type Engine struct {
	mu                sync.RWMutex
	batchMu           sync.Mutex
	bufferMu          sync.Mutex
	state             Snapshot
	buffer            []Operation
	equalsIndex       map[string]map[string]struct{}
	numberIndex       *BTree
	btreeDegree       int
	defaultTTLSeconds int64
	now               func() time.Time
}

func NewEngine() *Engine {
	return NewEngineWithConfig(defaultConfig())
}

func NewEngineWithConfig(config Config) *Engine {
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
	engine.rebuildIndexesLocked()
}

func (engine *Engine) Set(key string, value string) {
	engine.SetWithTTL(key, value, 0)
}

func (engine *Engine) SetWithTTL(key string, value string, ttlSeconds int64) StoredValue {
	engine.mu.Lock()
	defer engine.mu.Unlock()

	if ttlSeconds == 0 {
		ttlSeconds = engine.defaultTTLSeconds
	}

	stored := StoredValue{Value: value}
	if ttlSeconds > 0 {
		stored.ExpiresAt = engine.now().Add(time.Duration(ttlSeconds) * time.Second).UnixMilli()
	}

	previous, hadPrevious := engine.state[key]
	engine.state[key] = stored
	engine.updateIndexesLocked(key, previous, hadPrevious, stored)
	return stored
}

func (engine *Engine) Get(key string) (string, error) {
	value, _, err := engine.getAndExpire(key)
	return value, err
}

func (engine *Engine) getAndExpire(key string) (string, bool, error) {
	engine.mu.Lock()
	defer engine.mu.Unlock()

	stored, ok := engine.state[key]
	if !ok {
		return "", false, fmt.Errorf("key not found")
	}

	if engine.isExpired(stored) {
		delete(engine.state, key)
		engine.removeFromEqualsIndexLocked(key, stored.Value)
		engine.removeFromNumberIndexLocked(stored.Value)
		return "", true, fmt.Errorf("key not found")
	}

	return stored.Value, false, nil
}

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

func (engine *Engine) Entries() []Entry {
	entries, _ := engine.entriesAndExpiredWrites()
	return entries
}

func (engine *Engine) entriesAndExpiredWrites() ([]Entry, []Operation) {
	engine.mu.Lock()
	defer engine.mu.Unlock()

	writes := engine.removeExpiredLocked()
	entries := make([]Entry, 0, len(engine.state))
	for key, stored := range engine.state {
		entries = append(entries, Entry{Key: key, Value: stored.Value})
	}

	sortEntries(entries)
	return entries, writes
}

func (engine *Engine) Snapshot() Snapshot {
	engine.mu.Lock()
	defer engine.mu.Unlock()

	engine.removeExpiredLocked()
	snapshot := make(Snapshot, len(engine.state))
	for key, stored := range engine.state {
		snapshot[key] = stored
	}

	return snapshot
}

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

func (engine *Engine) ExecuteText(input string) (Result, []Operation) {
	command, err := ParseCommand(input)
	if err != nil {
		return Result{OK: false, Error: err.Error()}, nil
	}

	return engine.Execute(command)
}

func (engine *Engine) Execute(command Command) (Result, []Operation) {
	result, writes := engine.execute(command)
	engine.appendToBuffer(writes)
	return result, writes
}

func (engine *Engine) execute(command Command) (Result, []Operation) {
	switch command.Type {
	case CommandSet:
		stored := engine.SetWithTTL(command.Key, command.Value, command.TTLSeconds)
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
				return Result{OK: false, Error: err.Error()}, []Operation{{Type: CommandDelete, Key: command.Key}}
			}
			return Result{OK: false, Error: err.Error()}, nil
		}

		return Result{OK: true, Value: value}, nil
	case CommandWhere:
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

func (engine *Engine) appendToBuffer(writes []Operation) {
	if len(writes) == 0 {
		return
	}

	engine.bufferMu.Lock()
	defer engine.bufferMu.Unlock()
	engine.buffer = append(engine.buffer, writes...)
}

// DrainBuffer rend les écritures au worker puis remet la file à zéro.
func (engine *Engine) DrainBuffer() []Operation {
	engine.bufferMu.Lock()
	defer engine.bufferMu.Unlock()

	writes := make([]Operation, len(engine.buffer))
	copy(writes, engine.buffer)
	engine.buffer = engine.buffer[:0]
	return writes
}

func (engine *Engine) BufferSize() int {
	engine.bufferMu.Lock()
	defer engine.bufferMu.Unlock()
	return len(engine.buffer)
}

// ReplayOperations rejoue l'AOF sans remettre les opérations dans le buffer.
func (engine *Engine) ReplayOperations(operations []Operation) error {
	engine.mu.Lock()
	defer engine.mu.Unlock()

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

	engine.rebuildIndexesLocked()
	return nil
}

// Query exécute equals/contains par recherche simple et les plages via le B-Tree.
func (engine *Engine) Query(field FilterField, operator FilterOperator, expected string) ([]Entry, []Operation, error) {
	engine.mu.Lock()
	defer engine.mu.Unlock()

	writes := make([]Operation, 0)
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

	target, err := numberValue(expected)
	if err != nil {
		return nil, writes, fmt.Errorf("range filter value must be a number")
	}

	items := engine.numberIndex.RangeItems(operator, target)
	entries := make([]Entry, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		stored, ok := engine.state[item.Key]
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

// SweepExpired est appelé périodiquement par le worker.
func (engine *Engine) SweepExpired() []Operation {
	engine.mu.Lock()
	writes := engine.removeExpiredLocked()
	engine.mu.Unlock()
	engine.appendToBuffer(writes)
	return writes
}

func (engine *Engine) removeExpiredLocked() []Operation {
	writes := make([]Operation, 0)
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

func (engine *Engine) isExpired(stored StoredValue) bool {
	return stored.ExpiresAt > 0 && stored.ExpiresAt <= engine.now().UnixMilli()
}

func (engine *Engine) removeIfExpiredLocked(key string, stored StoredValue) bool {
	if !engine.isExpired(stored) {
		return false
	}

	delete(engine.state, key)
	engine.removeFromEqualsIndexLocked(key, stored.Value)
	engine.removeFromNumberIndexLocked(stored.Value)
	return true
}

// Cette reconstruction complète sert au restore. Les écritures normales mettent
// seulement à jour les éléments concernés avec updateIndexesLocked.
func (engine *Engine) rebuildIndexesLocked() {
	engine.equalsIndex = map[string]map[string]struct{}{}
	engine.numberIndex = NewBTree(engine.btreeDegree)

	for key, stored := range engine.state {
		keys := engine.equalsIndex[stored.Value]
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

func (engine *Engine) removeFromEqualsIndexLocked(key string, value string) {
	keys := engine.equalsIndex[value]
	delete(keys, key)
	if len(keys) == 0 {
		delete(engine.equalsIndex, value)
	}
}

func (engine *Engine) removeFromNumberIndexLocked(value string) {
	if _, err := numberValue(value); err == nil {
		engine.rebuildNumberIndexLocked()
	}
}

func (engine *Engine) rebuildNumberIndexLocked() {
	engine.numberIndex = NewBTree(engine.btreeDegree)
	for key, stored := range engine.state {
		if value, err := numberValue(stored.Value); err == nil {
			engine.numberIndex.Insert(BTreeItem{Value: value, Key: key})
		}
	}
}

func isCurrentNumberItem(value string, item BTreeItem) bool {
	current, err := numberValue(value)
	return err == nil && current == item.Value
}

func numberValue(value string) (float64, error) {
	if strings.HasPrefix(value, "n:") {
		value = strings.TrimPrefix(value, "n:")
	}
	return strconv.ParseFloat(value, 64)
}

func comparisonText(value string) string {
	for _, prefix := range []string{"s:", "n:", "b:", "j:"} {
		if strings.HasPrefix(value, prefix) {
			return strings.TrimPrefix(value, prefix)
		}
	}
	return value
}

func (engine *Engine) ExecuteBatch(commands []string) BatchResult {
	engine.batchMu.Lock()
	defer engine.batchMu.Unlock()

	results := make([]Result, 0, len(commands))
	writes := make([]Operation, 0)

	for _, commandText := range commands {
		result, commandWrites := engine.ExecuteText(commandText)
		results = append(results, result)
		writes = append(writes, commandWrites...)
	}

	return BatchResult{Results: results, Writes: writes, BufferSize: engine.BufferSize()}
}

func sortEntries(entries []Entry) {
	sort.Slice(entries, func(left int, right int) bool {
		return entries[left].Key < entries[right].Key
	})
}
