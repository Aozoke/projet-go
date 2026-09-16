package redis

import (
	"fmt"
	"testing"
)

// Ces mesures tournent en Go natif : elles n'incluent ni WASM, ni worker, ni disque, ni React.

// BenchmarkEngineSet mesure les ecritures repetees d'une valeur texte sous la meme cle.
// Il appelle Set directement : pas de parsing, de buffer ou de sauvegarde ici.
func BenchmarkEngineSet(b *testing.B) {
	engine := NewEngine()
	b.ResetTimer()

	for index := 0; index < b.N; index++ {
		engine.Set("benchmark", "value")
	}
}

// BenchmarkEngineGet mesure la lecture d'une cle existante, preparee avant le chronometre.
func BenchmarkEngineGet(b *testing.B) {
	engine := NewEngine()
	engine.Set("benchmark", "value")
	b.ResetTimer()

	for index := 0; index < b.N; index++ {
		_, _ = engine.Get("benchmark")
	}
}

// BenchmarkEngineWhere compare les recherches sur des bases de 100, 1 000 et 10 000 entrees.
// b.Run cree une mesure separee pour chaque filtre et chaque taille de base.
func BenchmarkEngineWhere(b *testing.B) {
	for _, size := range []int{100, 1_000, 10_000} {
		// equals cherche une seule valeur, celle situee au milieu des nombres generes.
		b.Run(fmt.Sprintf("equals/%d", size), func(b *testing.B) {
			engine := benchmarkEngine(size)
			wanted := fmt.Sprintf("%d", size/2)
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				_, _, _ = engine.Query(FilterValue, OperatorEquals, wanted)
			}
		})

		// contains parcourt les valeurs pour chercher le morceau de texte "99".
		b.Run(fmt.Sprintf("contains/%d", size), func(b *testing.B) {
			engine := benchmarkEngine(size)
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				_, _, _ = engine.Query(FilterValue, OperatorContains, "99")
			}
		})

		// Cette plage large renvoie environ la moitie de la base : beaucoup de resultats a construire.
		b.Run(fmt.Sprintf("range/%d", size), func(b *testing.B) {
			engine := benchmarkEngine(size)
			wanted := fmt.Sprintf("%d", size/2)
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				_, _, _ = engine.Query(FilterValue, OperatorGreaterOrEqual, wanted)
			}
		})

		// Cette plage etroite ne garde que le plus grand nombre, pour mesurer un parcours court.
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

// BenchmarkEngineBatch mesure un lot de quatre GET cote moteur, parsing compris.
// Il ne mesure pas le gain sur les messages worker : cela demande un benchmark navigateur.
func BenchmarkEngineBatch(b *testing.B) {
	engine := NewEngine()
	commands := []string{"GET benchmark", "GET benchmark", "GET benchmark", "GET benchmark"}
	engine.Set("benchmark", "value")
	b.ResetTimer()

	for index := 0; index < b.N; index++ {
		engine.ExecuteBatch(commands)
	}
}

// BenchmarkRestore mesure le chargement de 10 000 entrees, seul puis avec 1 000 operations rejouees.
// Snapshot et journal sont deja en memoire : aucune lecture de fichier n'est chronometree ici.
func BenchmarkRestore(b *testing.B) {
	const snapshotSize = 10_000
	source := benchmarkEngine(snapshotSize)
	snapshot := source.Snapshot()
	operations := make([]Operation, 1_000)
	// On prepare des SET avec des cles aof:... distinctes des cles key:... du snapshot.
	for index := range operations {
		operations[index] = Operation{
			Type:  CommandSet,
			Key:   fmt.Sprintf("aof:%d", index),
			Value: fmt.Sprintf("%d", index),
		}
	}

	// Chaque repetition cree un nouveau moteur, comme une reconstruction de RAM au demarrage.
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

// benchmarkEngine prepare les donnees des mesures : key:0 -> "0", key:1 -> "1", etc.
func benchmarkEngine(size int) *Engine {
	engine := NewEngine()
	for index := 0; index < size; index++ {
		engine.Set(fmt.Sprintf("key:%d", index), fmt.Sprintf("%d", index))
	}
	return engine
}
