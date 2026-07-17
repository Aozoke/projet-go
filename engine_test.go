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
