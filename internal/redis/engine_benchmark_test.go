package redis

import (
	"fmt"
	"testing"
)

func BenchmarkEngineSet(b *testing.B) {
	engine := NewEngine()
	b.ResetTimer()

	for index := 0; index < b.N; index++ {
		engine.Set("benchmark", fmt.Sprintf("%d", index))
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

func BenchmarkEngineWhereRange(b *testing.B) {
	engine := NewEngine()
	for index := 0; index < 100; index++ {
		engine.Set(fmt.Sprintf("key:%d", index), fmt.Sprintf("%d", index))
	}
	b.ResetTimer()

	for index := 0; index < b.N; index++ {
		_, _, _ = engine.Query(FilterValue, OperatorGreaterOrEqual, "50")
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
