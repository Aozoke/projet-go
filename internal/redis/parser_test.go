package redis

import "testing"

// Chaque test envoie un texte au parser puis compare la reponse a ce qu'on attend.

// TestReadCommandName verifie que seul le nom GET est extrait, sans la cle name.
func TestReadCommandName(t *testing.T) {
	result := ReadCommandName("GET name")

	if result != "GET" {
		t.Fatalf("expected GET, got %s", result)
	}
}

// TestReadCommandNameTrimsSpaces verifie que les espaces autour ne changent pas le nom.
func TestReadCommandNameTrimsSpaces(t *testing.T) {
	result := ReadCommandName("   GET name   ")

	if result != "GET" {
		t.Fatalf("expected GET, got %s", result)
	}
}

// TestReadCommandNameWithEmptyInput attend un nom vide quand aucun texte n'est fourni.
func TestReadCommandNameWithEmptyInput(t *testing.T) {
	result := ReadCommandName("")

	if result != "" {
		t.Fatalf("expected empty command name, got %s", result)
	}
}

// TestReadCommandNameUppercaseCommand verifie que get devient GET.
func TestReadCommandNameUppercaseCommand(t *testing.T) {
	result := ReadCommandName("get name")

	if result != "GET" {
		t.Fatalf("expected GET, got %s", result)
	}
}

// TestParseGetCommand attend une commande GET, la cle name et aucune erreur.
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

// TestParseGetCommandWithoutKey verifie que GET seul est refuse sans faire planter le parser.
func TestParseGetCommandWithoutKey(t *testing.T) {
	_, err := ParseCommand("GET")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestParseUnknownCommand verifie que PING, non gere par notre moteur, est refuse.
func TestParseUnknownCommand(t *testing.T) {
	_, err := ParseCommand("PING name")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestParseDeleteCommand verifie le type DELETE et la cle name, sans executer de suppression.
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

// TestParseDeleteCommandWithoutKey attend une erreur si DELETE n'a pas de cle.
func TestParseDeleteCommandWithoutKey(t *testing.T) {
	_, err := ParseCommand("DELETE")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestParseSetCommand verifie SET, la cle name et la valeur matt sans ses guillemets.
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

// TestParseSetCommandWithoutValue attend une erreur quand SET a une cle mais pas de valeur.
func TestParseSetCommandWithoutValue(t *testing.T) {
	_, err := ParseCommand("SET name")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestParseSetCommandWithSpacesInValue verifie que les deux mots hello world sont recuperes.
// Ce cas teste un espace simple, pas la conservation de plusieurs espaces consecutifs.
func TestParseSetCommandWithSpacesInValue(t *testing.T) {
	command, err := ParseCommand(`SET message "hello world"`)

	if err != nil {
		t.Fatal(err)
	}

	if command.Value != "hello world" {
		t.Fatalf("expected value hello world, got %s", command.Value)
	}
}

// TestParseSetCommandWithoutQuotes attend une erreur si matt n'est pas entre guillemets.
func TestParseSetCommandWithoutQuotes(t *testing.T) {
	_, err := ParseCommand("SET name matt")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestParseAllCommand verifie que ALL donne le type CommandAll sans erreur.
func TestParseAllCommand(t *testing.T) {
	command, err := ParseCommand("ALL")

	if err != nil {
		t.Fatal(err)
	}

	if command.Type != CommandAll {
		t.Fatalf("expected command type ALL, got %s", command.Type)
	}
}

// TestParseEmptyCommand verifie qu'un texte fait uniquement d'espaces est refuse.
func TestParseEmptyCommand(t *testing.T) {
	_, err := ParseCommand("   ")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestParseSetCommandWithTTL verifie que EX 60 remplit TTLSeconds avec le nombre 60.
// Le parser lit la duree ; ce test n'attend pas une expiration reelle.
func TestParseSetCommandWithTTL(t *testing.T) {
	command, err := ParseCommand(`SET session "active" EX 60`)
	if err != nil {
		t.Fatal(err)
	}

	if command.TTLSeconds != 60 {
		t.Fatalf("expected TTL 60, got %d", command.TTLSeconds)
	}
}

// TestParseWhereCommand verifie le champ value, l'operateur contains et le texte cherche.
func TestParseWhereCommand(t *testing.T) {
	command, err := ParseCommand(`GET WHERE value contains "hello world"`)
	if err != nil {
		t.Fatal(err)
	}

	if command.Type != CommandWhere || command.FilterField != FilterValue {
		t.Fatalf("unexpected command: %+v", command)
	}
	if command.Operator != OperatorContains || command.FilterValue != "hello world" {
		t.Fatalf("unexpected filter: %+v", command)
	}
}

// TestParseWhereCommandAcceptsSchemaField verifie qu'age peut etre un champ de filtre.
func TestParseWhereCommandAcceptsSchemaField(t *testing.T) {
	command, err := ParseCommand("GET WHERE age > 18")
	if err != nil {
		t.Fatal(err)
	}
	if command.FilterField != "age" {
		t.Fatalf("expected age field, got %s", command.FilterField)
	}
}

// TestParseSetCommandUnescapesJSON verifie que les \" deviennent des guillemets ordinaires.
// Le JSON reste du texte pour le moteur ; le parser retire seulement l'echappement du SET.
func TestParseSetCommandUnescapesJSON(t *testing.T) {
	command, err := ParseCommand(`SET user "{\"name\":\"matt\"}"`)
	if err != nil {
		t.Fatal(err)
	}
	if command.Value != `{"name":"matt"}` {
		t.Fatalf("unexpected value: %s", command.Value)
	}
}
