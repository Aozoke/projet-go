package main

import "testing"

// Vérifie qu'on récupère le premier mot d'une commande.
func TestReadCommandName(t *testing.T) {
	result := ReadCommandName("GET name")

	if result != "GET" {
		t.Fatalf("expected GET, got %s", result)
	}
}

// Vérifie que les espaces autour de la commande ne changent rien.
func TestReadCommandNameTrimsSpaces(t *testing.T) {
	result := ReadCommandName("   GET name   ")

	if result != "GET" {
		t.Fatalf("expected GET, got %s", result)
	}
}

// Vérifie qu'une commande vide ne fait pas crasher ReadCommandName.
func TestReadCommandNameWithEmptyInput(t *testing.T) {
	result := ReadCommandName("")

	if result != "" {
		t.Fatalf("expected empty command name, got %s", result)
	}
}

// Vérifie qu'une commande écrite en minuscules est normalisée en majuscules.
func TestReadCommandNameUppercaseCommand(t *testing.T) {
	result := ReadCommandName("get name")

	if result != "GET" {
		t.Fatalf("expected GET, got %s", result)
	}
}

// Vérifie que GET construit une Command avec un type et une clé.
func TestParseGetCommand(t *testing.T) {
	command, err := ParseCommand("GET name")

	if err != nil {
		t.Fatal(err)
	}

	if command.Type != CommandGet {
		t.Fatalf("expected command type GET, got %s", command.Type)
	}

	if command.Key != "name" {
		t.Fatalf("expected key name, got %s", command.Key)
	}
}

// Vérifie que GET sans clé renvoie une erreur propre.
func TestParseGetCommandWithoutKey(t *testing.T) {
	_, err := ParseCommand("GET")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// Vérifie qu'une commande inconnue est refusée.
func TestParseUnknownCommand(t *testing.T) {
	_, err := ParseCommand("PING name")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// Vérifie que DELETE construit une Command avec un type et une clé.
func TestParseDeleteCommand(t *testing.T) {
	command, err := ParseCommand("DELETE name")

	if err != nil {
		t.Fatal(err)
	}

	if command.Type != CommandDelete {
		t.Fatalf("expected command type DELETE, got %s", command.Type)
	}

	if command.Key != "name" {
		t.Fatalf("expected key name, got %s", command.Key)
	}
}

// Vérifie que DELETE sans clé renvoie une erreur propre.
func TestParseDeleteCommandWithoutKey(t *testing.T) {
	_, err := ParseCommand("DELETE")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// Vérifie que SET construit une Command avec un type, une clé et une valeur.
func TestParseSetCommand(t *testing.T) {
	command, err := ParseCommand(`SET name "matt"`)

	if err != nil {
		t.Fatal(err)
	}

	if command.Type != CommandSet {
		t.Fatalf("expected command type SET, got %s", command.Type)
	}

	if command.Key != "name" {
		t.Fatalf("expected key name, got %s", command.Key)
	}

	if command.Value != "matt" {
		t.Fatalf("expected value matt, got %s", command.Value)
	}
}

// Vérifie que SET sans valeur renvoie une erreur propre.
func TestParseSetCommandWithoutValue(t *testing.T) {
	_, err := ParseCommand("SET name")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// Vérifie que SET accepte une valeur contenant des espaces.
func TestParseSetCommandWithSpacesInValue(t *testing.T) {
	command, err := ParseCommand(`SET message "hello world"`)

	if err != nil {
		t.Fatal(err)
	}

	if command.Value != "hello world" {
		t.Fatalf("expected value hello world, got %s", command.Value)
	}
}

// Vérifie que la valeur de SET doit être entre guillemets.
func TestParseSetCommandWithoutQuotes(t *testing.T) {
	_, err := ParseCommand("SET name matt")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// Vérifie qu'une commande vide renvoie une erreur.
func TestParseEmptyCommand(t *testing.T) {
	_, err := ParseCommand("   ")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
