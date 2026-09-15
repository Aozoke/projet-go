package redis

import (
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	engine := NewEngine()

	if engine == nil {
		t.Fatal("expected engine, got nil")
	}

	if engine.state == nil {
		t.Fatal("expected state, got nil")
	}
}

func TestEngineSetAndGet(t *testing.T) {
	engine := NewEngine()

	engine.Set("name", "matt")

	value, err := engine.Get("name")
	if err != nil {
		t.Fatal(err)
	}

	if value != "matt" {
		t.Fatalf("expected matt, got %s", value)
	}
}

func TestEngineGetMissingKey(t *testing.T) {
	engine := NewEngine()

	_, err := engine.Get("missing")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestEngineDelete(t *testing.T) {
	engine := NewEngine()

	engine.Set("name", "matt")
	engine.Delete("name")

	_, err := engine.Get("name")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestEngineExecuteBatch(t *testing.T) {
	engine := NewEngine()

	result := engine.ExecuteBatch([]string{
		`SET name "matt"`,
		"GET name",
		"DELETE name",
		"GET name",
	})

	if len(result.Results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(result.Results))
	}

	if !result.Results[0].OK || result.Results[1].Value != "matt" || !result.Results[2].OK || result.Results[3].OK {
		t.Fatalf("unexpected batch result: %+v", result.Results)
	}

	if len(result.Writes) != 2 {
		t.Fatalf("expected 2 write operations, got %d", len(result.Writes))
	}
	if result.BufferSize != 2 {
		t.Fatalf("expected 2 buffered operations, got %d", result.BufferSize)
	}
}

func TestEngineDrainBuffer(t *testing.T) {
	engine := NewEngine()
	engine.ExecuteBatch([]string{`SET name "matt"`, "DELETE name"})

	writes := engine.DrainBuffer()
	if len(writes) != 2 || engine.BufferSize() != 0 {
		t.Fatalf("unexpected drained buffer: %+v", writes)
	}
	if empty := engine.DrainBuffer(); empty == nil || len(empty) != 0 {
		t.Fatalf("empty buffer must be an empty array, got %+v", empty)
	}
}

func TestReplayOperationsRestoresStateWithoutBuffering(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	engine := NewEngineWithConfig(Config{BTreeDegree: 4, Now: func() time.Time { return now }})
	expiresAt := now.Add(time.Minute).UnixMilli()

	err := engine.ReplayOperations([]Operation{
		{Type: CommandSet, Key: "name", Value: "s:matt"},
		{Type: CommandSet, Key: "session", Value: "s:active", ExpiresAt: expiresAt},
		{Type: CommandDelete, Key: "name"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if engine.BufferSize() != 0 {
		t.Fatal("replay must not fill the write buffer")
	}
	if _, err := engine.Get("name"); err == nil {
		t.Fatal("deleted key was restored")
	}
	if value, err := engine.Get("session"); err != nil || value != "s:active" {
		t.Fatalf("TTL value was not restored: %q, %v", value, err)
	}
}

func TestEngineEntries(t *testing.T) {
	engine := NewEngine()

	engine.Set("b", "2")
	engine.Set("a", "1")

	entries := engine.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if entries[0].Key != "a" || entries[1].Key != "b" {
		t.Fatalf("entries are not sorted: %+v", entries)
	}
}

func TestEngineWhereEqualsAndContains(t *testing.T) {
	engine := NewEngine()
	engine.Set("first", "hello world")
	engine.Set("second", "other")

	equals, _, err := engine.Query(FilterValue, OperatorEquals, "other")
	if err != nil || len(equals) != 1 || equals[0].Key != "second" {
		t.Fatalf("unexpected equals result: %+v, %v", equals, err)
	}

	contains, _, err := engine.Query(FilterValue, OperatorContains, "world")
	if err != nil || len(contains) != 1 || contains[0].Key != "first" {
		t.Fatalf("unexpected contains result: %+v, %v", contains, err)
	}
}

func TestEngineWhereRangeUsesNumberIndex(t *testing.T) {
	engine := NewEngine()
	engine.Set("young", "17")
	engine.Set("adult", "25")
	engine.Set("senior", "70")

	entries, _, err := engine.Query(FilterValue, OperatorGreaterOrEqual, "18")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Key != "adult" || entries[1].Key != "senior" {
		t.Fatalf("unexpected range result: %+v", entries)
	}
}

func TestEngineWhereUsesSchemaField(t *testing.T) {
	engine := NewEngine()
	engine.Set("name", "s:matt")
	engine.Set("age", "n:25")

	entries, _, err := engine.Query("age", OperatorGreaterThan, "n:18")
	if err != nil || len(entries) != 1 || entries[0].Key != "age" {
		t.Fatalf("unexpected schema field result: %+v, %v", entries, err)
	}
}

func TestEngineRangeIgnoresUpdatedAndDeletedValues(t *testing.T) {
	engine := NewEngine()
	engine.Set("score", "50")
	engine.Set("score", "5")
	engine.Set("deleted", "60")
	engine.Delete("deleted")

	entries, _, err := engine.Query(FilterValue, OperatorGreaterThan, "40")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("stale B-Tree values leaked into result: %+v", entries)
	}
	indexed := engine.numberIndex.RangeItems(OperatorGreaterThan, -1)
	if len(indexed) != 1 || indexed[0].Key != "score" || indexed[0].Value != 5 {
		t.Fatalf("stale values remain in B-Tree: %+v", indexed)
	}
}

func TestEngineTTLExpiresOnGet(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	engine := NewEngineWithConfig(Config{
		BTreeDegree: 4,
		Now:         func() time.Time { return now },
	})

	engine.SetWithTTL("session", "active", 10)
	now = now.Add(11 * time.Second)

	result, writes := engine.ExecuteText("GET session")
	if result.OK {
		t.Fatal("expected expired key to be missing")
	}
	if len(writes) != 1 || writes[0].Type != CommandDelete {
		t.Fatalf("expected persisted delete, got %+v", writes)
	}
}

func TestEngineTTLSweep(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	engine := NewEngineWithConfig(Config{
		BTreeDegree: 4,
		Now:         func() time.Time { return now },
	})

	engine.SetWithTTL("temporary", "value", 5)
	now = now.Add(6 * time.Second)
	writes := engine.SweepExpired()

	if len(writes) != 1 || writes[0].Key != "temporary" {
		t.Fatalf("unexpected sweep writes: %+v", writes)
	}
}

func TestFilteredGetPersistsExpiredDelete(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	engine := NewEngineWithConfig(Config{BTreeDegree: 4, Now: func() time.Time { return now }})
	engine.SetWithTTL("score", "50", 5)
	now = now.Add(6 * time.Second)

	result, writes := engine.ExecuteText("GET WHERE value > 10")
	if !result.OK || len(result.Entries) != 0 {
		t.Fatalf("expired entry leaked into filter: %+v", result)
	}
	if len(writes) != 1 || writes[0].Type != CommandDelete {
		t.Fatalf("expected persisted expiration, got %+v", writes)
	}
}

func TestSnapshotKeepsTTL(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	config := Config{BTreeDegree: 4, Now: func() time.Time { return now }}
	firstEngine := NewEngineWithConfig(config)
	firstEngine.SetWithTTL("session", "active", 60)

	secondEngine := NewEngineWithConfig(config)
	secondEngine.LoadSnapshot(firstEngine.Snapshot())

	value, err := secondEngine.Get("session")
	if err != nil || value != "active" {
		t.Fatalf("snapshot did not restore TTL value: %q, %v", value, err)
	}

	now = now.Add(61 * time.Second)
	if _, err := secondEngine.Get("session"); err == nil {
		t.Fatal("expected restored key to expire")
	}
}

func TestDefaultTTL(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	engine := NewEngineWithConfig(Config{
		BTreeDegree:       4,
		DefaultTTLSeconds: 5,
		Now:               func() time.Time { return now },
	})

	engine.Set("temporary", "value")
	now = now.Add(6 * time.Second)
	if _, err := engine.Get("temporary"); err == nil {
		t.Fatal("expected default TTL to expire the key")
	}
}
