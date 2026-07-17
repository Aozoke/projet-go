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

func TestReadCommandNameUppercaseCommand(t *testing.T) {
	//On teste une commande écrite en minuscules.
	result := ReadCommandName("get name")

	if result != "GET" {
		//On veut que notre parser normalise en majuscules.
		t.Fatalf("expected GET, got %s", result)
	}
}

// On appelle une future fonction ParseCommand. renvoie comand +err
func TestParseGetCommand(t *testing.T) {
	command, err := ParseCommand("GET name")
	//t.Logf("%+v", command) <- pour avoir le tests en command

	if err != nil {
		t.Fatal(err)
		//si err alors que get name valide, test echoue
	}

	//On vérifie que le type est bien GET.
	if command.Type != CommandGet {
		t.Fatalf("expected command type GET, got %s", command.Type)
	}

	if command.Key != "name" {
		t.Fatalf("expected key name, got %s", command.Key)
	}
}

func TestParseGetCommandWithoutKey(t *testing.T) {
	//on ignore la commande avec _. parce que dans ce cas on veut juste l'erreur
	_, err := ParseCommand("GET")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// refuser les commandes inconues
// test pour une commande inconnue
func TestParseUnknownCommand(t *testing.T) {
	//On appelle le parser avec PING name
	_, err := ParseCommand("PING name")

	//Si err est nil = le parser a accepté la commande
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	//et c'est pas bon donc fail
}

func TestParseDeleteCommand(t *testing.T) {
	//On demande au parser de lire la commande
	command, err := ParseCommand("DELETE name")

	//si err n'est pas vide
	if err != nil {
		//si ee le test s'arrete tout de suite diff fatalf et fatal fatalf a un msgg formater
		t.Fatal(err)
	}

	if command.Type != CommandDelete {
		//si le type est bien delete
		//on affiche une string, si renvoie get au lieu de delete, le message affiche ce qu'il recoi
		t.Fatalf("expected command type DELETE, got %s", command.Type)
	}

	if command.Key != "name" {
		//on verifie que la cle et bien name
		//si clef mauvaise, le test echoue avec un message clair
		t.Fatalf("expected key name, got %s", command.Key)
	}
}

func TestParseDeleteCommandWithoutKey(t *testing.T) {
	// _ = ignore la valeur  parce que parser renvoie command et err et dans le test on veux vérifier que l'err
	_, err := ParseCommand("DELETE")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

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
