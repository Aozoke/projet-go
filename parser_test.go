package main

import "testing"

// fonction de test, toujours commencer par Test
func TestReadCommandName(t *testing.T) {
	//appel la fonction avec "GET name".
	result := ReadCommandName("GET name")

	if result != "GET" {
		//on vérifie si resultat est diff
		t.Fatalf("expected GET, got %s", result)

		//si ce n'est pas get on arrête le test
	}
}

// dans l'idée pareil

func TestReadCommandNameTrimsSpaces(t *testing.T) {
	result := ReadCommandName("   GET name   ")

	if result != "GET" {
		t.Fatalf("expected GET, got %s", result)
	}
}

func TestReadCommandNameWithEmptyInput(t *testing.T) {
	result := ReadCommandName("")

	if result != "" {
		t.Fatalf("expected empty command name, got %s", result)
	}
}
