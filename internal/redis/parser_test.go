package redis

import "testing"

func TestReadCommandName(t *testing.T) {
	result := ReadCommandName("GET name")

	if result != "GET" {
		t.Fatalf("expected GET, got %s", result)
	}
}

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
	result := ReadCommandName("get name")

	if result != "GET" {
		t.Fatalf("expected GET, got %s", result)
	}
}

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

func TestParseGetCommandWithoutKey(t *testing.T) {
	_, err := ParseCommand("GET")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseUnknownCommand(t *testing.T) {
	_, err := ParseCommand("PING name")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

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

func TestParseDeleteCommandWithoutKey(t *testing.T) {
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

func TestParseSetCommandWithoutValue(t *testing.T) {
	_, err := ParseCommand("SET name")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseSetCommandWithSpacesInValue(t *testing.T) {
	command, err := ParseCommand(`SET message "hello world"`)

	if err != nil {
		t.Fatal(err)
	}

	if command.Value != "hello world" {
		t.Fatalf("expected value hello world, got %s", command.Value)
	}
}

func TestParseSetCommandWithoutQuotes(t *testing.T) {
	_, err := ParseCommand("SET name matt")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAllCommand(t *testing.T) {
	command, err := ParseCommand("ALL")

	if err != nil {
		t.Fatal(err)
	}

	if command.Type != CommandAll {
		t.Fatalf("expected command type ALL, got %s", command.Type)
	}
}

func TestParseEmptyCommand(t *testing.T) {
	_, err := ParseCommand("   ")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseSetCommandWithTTL(t *testing.T) {
	command, err := ParseCommand(`SET session "active" EX 60`)
	if err != nil {
		t.Fatal(err)
	}

	if command.TTLSeconds != 60 {
		t.Fatalf("expected TTL 60, got %d", command.TTLSeconds)
	}
}

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

func TestParseWhereCommandRejectsUnknownField(t *testing.T) {
	_, err := ParseCommand("GET WHERE age > 18")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
