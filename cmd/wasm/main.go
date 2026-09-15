//go:build js && wasm

// sert de pont entre JavaScript et le moteur Go dans le navigateur.

package main

import (
	// Permet de convertir du JSON en Go et inversement
	"encoding/json"

	// Permet au code Go compilé en WASM de communiquer avec JavaScript
	"syscall/js"

	// Importe notre moteur Redis
	"github.com/Aozoke/projet-go/internal/redis"
)

// Structure du message reçu pour exécuter une ou plusieurs commandes
type executeRequest struct {
	// Liste des commandes à exécuter
	Commands []string `json:"commands"`
}

type configurationRequest struct {
	DefaultTTLSeconds int64 `json:"defaultTTLSeconds"`
	BTreeDegree       int   `json:"btreeDegree"`
}

// Structure utilisée pour renvoyer une erreur ou un statut
type errorResponse struct {
	// Indique si tout s'est bien passé
	OK bool `json:"ok"`

	// Contient le message d'erreur si besoin
	Error string `json:"error"`
}

// Création du moteur Redis utilisé par le WASM
var engine = redis.NewEngine()

// Point d'entrée du programme WASM
func main() {

	// Rend la fonction wasmRedisExecute accessible depuis JavaScript
	js.Global().Set("wasmRedisExecute", js.FuncOf(wasmRedisExecute))

	// Rend la fonction de chargement d'un snapshot accessible depuis JavaScript
	js.Global().Set("wasmRedisLoadSnapshot", js.FuncOf(wasmRedisLoadSnapshot))

	// Rend la fonction de création d'un snapshot accessible depuis JavaScript
	js.Global().Set("wasmRedisDumpSnapshot", js.FuncOf(wasmRedisDumpSnapshot))

	// Permet au worker d'envoyer la configuration issue du fichier .env.
	js.Global().Set("wasmRedisConfigure", js.FuncOf(wasmRedisConfigure))

	// Permet au worker de lancer le balayage périodique des TTL.
	js.Global().Set("wasmRedisSweepExpired", js.FuncOf(wasmRedisSweepExpired))
	js.Global().Set("wasmRedisDrainBuffer", js.FuncOf(wasmRedisDrainBuffer))
	js.Global().Set("wasmRedisReplayOperations", js.FuncOf(wasmRedisReplayOperations))

	// Empêche le programme WASM de se terminer
	select {}
}

// Fonction appelée depuis JavaScript pour exécuter des commandes Redis
func wasmRedisExecute(_ js.Value, args []js.Value) any {

	// Vérifie qu'un argument a bien été envoyé
	if len(args) == 0 {

		// Renvoie une erreur si aucune requête n'est reçue
		return encode(errorResponse{OK: false, Error: "missing request"})
	}

	// Variable qui va contenir la requête reçue
	var request executeRequest

	// Transforme le JSON reçu depuis JavaScript en structure Go
	if err := json.Unmarshal([]byte(args[0].String()), &request); err != nil {

		// Renvoie l'erreur si le JSON n'est pas valide
		return encode(errorResponse{OK: false, Error: err.Error()})
	}

	// Exécute toutes les commandes puis renvoie le résultat en JSON
	return encode(engine.ExecuteBatch(request.Commands))
}

// Fonction appelée pour recharger un snapshot dans le moteur
func wasmRedisLoadSnapshot(_ js.Value, args []js.Value) any {

	// Vérifie qu'un snapshot a bien été envoyé
	if len(args) == 0 {

		// Renvoie une erreur si aucun snapshot n'est reçu
		return encode(errorResponse{OK: false, Error: "missing snapshot"})
	}

	snapshot, err := decodeSnapshot([]byte(args[0].String()))
	if err != nil {

		// Renvoie l'erreur si le JSON est invalide
		return encode(errorResponse{OK: false, Error: err.Error()})
	}

	// Charge les données du snapshot dans le moteur Redis
	engine.LoadSnapshot(snapshot)

	// Confirme que le chargement s'est bien passé
	return encode(errorResponse{OK: true})
}

func wasmRedisConfigure(_ js.Value, args []js.Value) any {
	if len(args) == 0 {
		return encode(errorResponse{OK: false, Error: "missing configuration"})
	}

	var configuration configurationRequest
	if err := json.Unmarshal([]byte(args[0].String()), &configuration); err != nil {
		return encode(errorResponse{OK: false, Error: err.Error()})
	}
	if configuration.DefaultTTLSeconds < 0 || configuration.BTreeDegree < 2 {
		return encode(errorResponse{OK: false, Error: "invalid configuration"})
	}

	engine.Configure(configuration.DefaultTTLSeconds, configuration.BTreeDegree)
	return encode(errorResponse{OK: true})
}

func wasmRedisSweepExpired(_ js.Value, _ []js.Value) any {
	writes := engine.SweepExpired()
	return encode(redis.BatchResult{
		Results:    []redis.Result{},
		Writes:     writes,
		BufferSize: engine.BufferSize(),
	})
}

func wasmRedisDrainBuffer(_ js.Value, _ []js.Value) any {
	return encode(engine.DrainBuffer())
}

func wasmRedisReplayOperations(_ js.Value, args []js.Value) any {
	if len(args) == 0 {
		return encode(errorResponse{OK: false, Error: "missing operations"})
	}

	var operations []redis.Operation
	if err := json.Unmarshal([]byte(args[0].String()), &operations); err != nil {
		return encode(errorResponse{OK: false, Error: err.Error()})
	}
	if err := engine.ReplayOperations(operations); err != nil {
		return encode(errorResponse{OK: false, Error: err.Error()})
	}

	return encode(errorResponse{OK: true})
}

// Fonction appelée pour récupérer l'état actuel de la base
func wasmRedisDumpSnapshot(_ js.Value, _ []js.Value) any {

	// Récupère le snapshot du moteur et le transforme en JSON
	return encode(engine.Snapshot())
}

// decodeSnapshot accepte le nouveau format avec TTL et l'ancien format string.
func decodeSnapshot(data []byte) (redis.Snapshot, error) {
	var snapshot redis.Snapshot
	if err := json.Unmarshal(data, &snapshot); err == nil {
		return snapshot, nil
	}

	var legacy map[string]string
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, err
	}

	snapshot = make(redis.Snapshot, len(legacy))
	for key, value := range legacy {
		snapshot[key] = redis.StoredValue{Value: value}
	}
	return snapshot, nil
}

// Fonction utilitaire pour transformer une valeur Go en JSON
func encode(value any) string {

	// Convertit la valeur Go en JSON
	bytes, err := json.Marshal(value)

	// Vérifie si la conversion a échoué
	if err != nil {

		// Crée une réponse d'erreur de secours
		fallback, _ := json.Marshal(
			errorResponse{
				OK:    false,
				Error: err.Error(),
			},
		)

		// Renvoie l'erreur sous forme de texte JSON
		return string(fallback)
	}

	// Renvoie le JSON sous forme de chaîne de caractères
	return string(bytes)
}
