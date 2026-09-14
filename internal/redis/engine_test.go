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
