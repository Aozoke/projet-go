package main

import (
	"fmt"
	"strings"
)

// ReadCommandName lit le nom de la commande.
// Exemple : "get name" devient "GET".
func ReadCommandName(input string) string {
	trimmed := strings.TrimSpace(input) //Ça enlève les espaces au début et à la fin.
	parts := strings.Fields(trimmed)    //Ça découpe la string en morceaux séparés par les espaces.

	if len(parts) == 0 {
		return ""
	}

	return strings.ToUpper(parts[0])
}

// ParseCommand transforme une commande texte en Command.
// Si la commande est invalide, elle renvoie une erreur.
func ParseCommand(input string) (Command, error) {
	// parts contient les morceaux de la commande.
	// Exemple : `SET name "matt"` devient ["SET", "name", "\"matt\""].
	parts := strings.Fields(strings.TrimSpace(input))
	commandName := ReadCommandName(input)

	//Si commandName n’est pas GET, et n’est pas DELETE, et n’est pas SET, alors la commande est inconnue.
	if commandName != string(CommandGet) && commandName != string(CommandDelete) && commandName != string(CommandSet) {
		return Command{}, fmt.Errorf("unknown command")
	}

	// GET, DELETE et SET ont tous besoin d'une clé.
	if len(parts) < 2 {
		return Command{}, fmt.Errorf("missing key")
	}

	value := ""
	if commandName == string(CommandSet) {
		// SET a besoin d'un troisième morceau : la valeur.
		if len(parts) < 3 {
			return Command{}, fmt.Errorf("missing value")
		}

		// On recolle tout ce qui vient après la clé pour garder les espaces.
		rawValue := strings.Join(parts[2:], " ")

		// Dans notre syntaxe, la valeur d'un SET doit être entre guillemets.
		if !strings.HasPrefix(rawValue, `"`) || !strings.HasSuffix(rawValue, `"`) {
			return Command{}, fmt.Errorf("value must be quoted")
		}

		// On stocke la valeur sans les guillemets autour.
		value = strings.Trim(rawValue, `"`)
	}

	return Command{
		Type:  CommandType(commandName),
		Key:   parts[1],
		Value: value,
	}, nil
}
