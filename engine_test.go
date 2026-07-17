package main

import "testing"

func TestNewEngine(t *testing.T) {
	// crée un moteur
	engine := NewEngine()

	if engine == nil {
		t.Fatal("expected engine, got nil")
	}

	//vérifie que map state existe bien
	if engine.state == nil {
		t.Fatal("expected state, got nil")
	}
}

func TestEngineSet(t *testing.T) {
	engine := NewEngine()

	//SET écrire / stocker une valeur. Stocke la valeur matt dans la clé name
	//On demande au moteur de stocker la valeur matt dans la clé name.
	//Dans le moteur, mets name -> matt
	engine.Set("name", "matt")

	//state, c’est l’état actuel de la base en mémoire.
	//regarder dans la map si la valeur a bien été stockée.
	//Si ce qu’il y a dans state["name"] n’est pas matt, le test échoue.
	if engine.state["name"] != "matt" {
		t.Fatalf("expected matt, got %s", engine.state["name"])
	}
}
