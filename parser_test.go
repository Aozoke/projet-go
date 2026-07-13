package main

import "testing"

func TestReadCommandName(t *testing.T) {
	result := ReadCommandName("GET name")

	if result != "GET" {
		t.Fatalf("expected GET, got %s", result)
	}
}
