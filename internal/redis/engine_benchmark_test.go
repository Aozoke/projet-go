package redis

import (
	"fmt"
	"testing"
)

func BenchmarkEngineSet(b *testing.B) {
	engine := NewEngine()
	b.ResetTimer()

	for index := 0; index < b.N; index++ {
		engine.Set("benchmark", "value")
	}
}

func BenchmarkEngineGet(b *testing.B) {
	engine := NewEngine()
	engine.Set("benchmark", "value")
	b.ResetTimer()

	for index := 0; index < b.N; index++ {
		_, _ = engine.Get("benchmark")
	}
}

func BenchmarkEngineWhere(b *testing.B) {
	for _, size := range []int{100, 1_000, 10_000} {
		b.Run(fmt.Sprintf("equals/%d", size), func(b *testing.B) {
			engine := benchmarkEngine(size)
			wanted := fmt.Sprintf("%d", size/2)
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				_, _, _ = engine.Query(FilterValue, OperatorEquals, wanted)
			}
		})

		b.Run(fmt.Sprintf("contains/%d", size), func(b *testing.B) {
			engine := benchmarkEngine(size)
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				_, _, _ = engine.Query(FilterValue, OperatorContains, "99")
			}
		})

		b.Run(fmt.Sprintf("range/%d", size), func(b *testing.B) {
			engine := benchmarkEngine(size)
			wanted := fmt.Sprintf("%d", size/2)
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				_, _, _ = engine.Query(FilterValue, OperatorGreaterOrEqual, wanted)
			}
		})

		b.Run(fmt.Sprintf("range-selective/%d", size), func(b *testing.B) {
			engine := benchmarkEngine(size)
			wanted := fmt.Sprintf("%d", size-2)
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				_, _, _ = engine.Query(FilterValue, OperatorGreaterThan, wanted)
			}
		})
	}
}

func BenchmarkEngineBatch(b *testing.B) {
	engine := NewEngine()
	commands := []string{"GET benchmark", "GET benchmark", "GET benchmark", "GET benchmark"}
	engine.Set("benchmark", "value")
	b.ResetTimer()

	for index := 0; index < b.N; index++ {
		engine.ExecuteBatch(commands)
	}
}

func BenchmarkRestore(b *testing.B) {
	const snapshotSize = 10_000
	source := benchmarkEngine(snapshotSize)
	snapshot := source.Snapshot()
	operations := make([]Operation, 1_000)
	for index := range operations {
		operations[index] = Operation{
			Type:  CommandSet,
			Key:   fmt.Sprintf("aof:%d", index),
			Value: fmt.Sprintf("%d", index),
		}
	}

	b.Run("snapshot-seul", func(b *testing.B) {
		for index := 0; index < b.N; index++ {
			engine := NewEngine()
			engine.LoadSnapshot(snapshot)
		}
	})

	b.Run("snapshot-et-aof", func(b *testing.B) {
		for index := 0; index < b.N; index++ {
			engine := NewEngine()
			engine.LoadSnapshot(snapshot)
			if err := engine.ReplayOperations(operations); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func benchmarkEngine(size int) *Engine {
	engine := NewEngine()
	for index := 0; index < size; index++ {
		engine.Set(fmt.Sprintf("key:%d", index), fmt.Sprintf("%d", index))
	}
	return engine
}
