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
	state             Snapshot
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

	engine.state[key] = stored
	engine.rebuildIndexesLocked()
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
		engine.rebuildIndexesLocked()
		return "", true, fmt.Errorf("key not found")
	}

	return stored.Value, false, nil
}

func (engine *Engine) Delete(key string) {
	engine.mu.Lock()
	defer engine.mu.Unlock()

	delete(engine.state, key)
	engine.rebuildIndexesLocked()
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

// Query exécute equals/contains par recherche simple et les plages via le B-Tree.
func (engine *Engine) Query(field FilterField, operator FilterOperator, expected string) ([]Entry, []Operation, error) {
	engine.mu.Lock()
	defer engine.mu.Unlock()

	writes := engine.removeExpiredLocked()
	if operator == OperatorEquals {
		entries := engine.queryEqualsLocked(field, expected)
		sortEntries(entries)
		return entries, writes, nil
	}

	if operator == OperatorContains {
		entries := engine.queryContainsLocked(field, expected)
		sortEntries(entries)
		return entries, writes, nil
	}

	if field != FilterValue {
		return nil, writes, fmt.Errorf("range filters only work on value")
	}

	target, err := strconv.ParseFloat(expected, 64)
	if err != nil {
		return nil, writes, fmt.Errorf("range filter value must be a number")
	}

	keys := engine.numberIndex.Range(operator, target)
	entries := make([]Entry, 0, len(keys))
	for _, key := range keys {
		stored, ok := engine.state[key]
		if ok {
			entries = append(entries, Entry{Key: key, Value: stored.Value})
		}
	}

	sortEntries(entries)
	return entries, writes, nil
}

func (engine *Engine) queryEqualsLocked(field FilterField, expected string) []Entry {
	if field == FilterKey {
		stored, ok := engine.state[expected]
		if !ok {
			return nil
		}
		return []Entry{{Key: expected, Value: stored.Value}}
	}

	keys := engine.equalsIndex[expected]
	entries := make([]Entry, 0, len(keys))
	for key := range keys {
		entries = append(entries, Entry{Key: key, Value: engine.state[key].Value})
	}
	return entries
}

func (engine *Engine) queryContainsLocked(field FilterField, expected string) []Entry {
	entries := make([]Entry, 0)
	for key, stored := range engine.state {
		text := stored.Value
		if field == FilterKey {
			text = key
		}
		if strings.Contains(text, expected) {
			entries = append(entries, Entry{Key: key, Value: stored.Value})
		}
	}
	return entries
}

// SweepExpired est appelé périodiquement par le worker.
func (engine *Engine) SweepExpired() []Operation {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	return engine.removeExpiredLocked()
}

func (engine *Engine) removeExpiredLocked() []Operation {
	writes := make([]Operation, 0)
	for key, stored := range engine.state {
		if engine.isExpired(stored) {
			delete(engine.state, key)
			writes = append(writes, Operation{Type: CommandDelete, Key: key})
		}
	}

	if len(writes) > 0 {
		engine.rebuildIndexesLocked()
	}
	return writes
}

func (engine *Engine) isExpired(stored StoredValue) bool {
	return stored.ExpiresAt > 0 && stored.ExpiresAt <= engine.now().UnixMilli()
}

// Pour garder une première version lisible, les index sont reconstruits après
// chaque écriture. On pourra ajouter une suppression B-Tree optimisée plus tard.
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

		value, err := strconv.ParseFloat(stored.Value, 64)
		if err == nil {
			engine.numberIndex.Insert(BTreeItem{Value: value, Key: key})
		}
	}
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

	return BatchResult{Results: results, Writes: writes}
}

func sortEntries(entries []Entry) {
	sort.Slice(entries, func(left int, right int) bool {
		return entries[left].Key < entries[right].Key
	})
}
