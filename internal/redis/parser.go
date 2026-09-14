// sert à transformer une commande texte en une structure que le moteur Go peut comprendre.

package redis

import (
	// Permet de créer des messages d'erreur
	"fmt"
	"strconv"

	// Permet de manipuler les chaînes de caractères
	"strings"
)

// Lit le premier mot de la commande.
// Exemple : "get name" devient "GET".
func ReadCommandName(input string) string {

	// Enlève les espaces inutiles au début et à la fin
	trimmed := strings.TrimSpace(input)

	// Découpe la commande en plusieurs mots
	parts := strings.Fields(trimmed)

	// Si la commande est vide, on renvoie une chaîne vide
	if len(parts) == 0 {
		return ""
	}

	// Retourne le premier mot en majuscules
	return strings.ToUpper(parts[0])
}

// Transforme une commande texte en structure Command.
// Si la commande est invalide, une erreur est renvoyée.
func ParseCommand(input string) (Command, error) {

	// Nettoie puis découpe la commande en plusieurs parties - Le parser récupère le texte et le découpe -
	parts := strings.Fields(strings.TrimSpace(input))

	// Le parser regarde quel type de commande c’est
	commandName := ReadCommandName(input)

	// On regarde si c'est SET, GET, DELETE ou ALL.
	switch CommandType(commandName) {

	// GET accepte soit une cle, soit la forme GET WHERE.
	case CommandGet:
		if len(parts) > 1 && strings.EqualFold(parts[1], "WHERE") {
			return parseWhereCommand(parts)
		}
		return parseKeyCommand(CommandGet, parts)

	// DELETE fonctionne de la même manière que GET
	case CommandDelete:
		return parseKeyCommand(CommandDelete, parts)

	// SET a besoin d'une clé et d'une valeur
	case CommandSet:
		return parseSetCommand(parts)

	// ALL ne prend aucun argument
	case CommandAll:
		return parseAllCommand(parts)

	// Si la commande n'existe pas, on renvoie une erreur
	default:
		return Command{}, fmt.Errorf("unknown command")
	}
}

// Parse les commandes qui prennent seulement une clé.
// Utilisé pour GET et DELETE.
func parseKeyCommand(commandType CommandType, parts []string) (Command, error) {

	// Vérifie qu'une clé a bien été donnée
	if len(parts) < 2 {
		return Command{}, fmt.Errorf("missing key")
	}

	// Vérifie qu'il n'y a pas trop d'arguments
	if len(parts) > 2 {
		return Command{}, fmt.Errorf("too many arguments")
	}

	// Création de la commande avec son type et sa clé
	return Command{
		Type: commandType,
		Key:  parts[1],
	}, nil
}

// Parse la commande SET
func parseSetCommand(parts []string) (Command, error) {

	// Vérifie qu'une clé est présente
	if len(parts) < 2 {
		return Command{}, fmt.Errorf("missing key")
	}

	// Vérifie qu'une valeur est présente
	if len(parts) < 3 {
		return Command{}, fmt.Errorf("missing value")
	}

	valueEnd := len(parts)
	ttlSeconds := int64(0)

	// EX suivi d'un nombre ajoute une durée de vie à la clé.
	if len(parts) >= 5 && strings.EqualFold(parts[len(parts)-2], "EX") {
		parsedTTL, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
		if err != nil || parsedTTL <= 0 {
			return Command{}, fmt.Errorf("TTL must be a positive number")
		}

		ttlSeconds = parsedTTL
		valueEnd -= 2
	}

	// Regroupe tout ce qui se trouve après la clé et avant EX.
	// Cela permet d'avoir une valeur avec plusieurs mots
	rawValue := strings.Join(parts[2:valueEnd], " ")

	// Vérifie que la valeur est entourée de guillemets
	if !strings.HasPrefix(rawValue, `"`) || !strings.HasSuffix(rawValue, `"`) {
		return Command{}, fmt.Errorf("value must be quoted")
	}

	// Création de la commande SET
	return Command{
		Type: CommandSet,
		Key:  parts[1],

		// Retire les guillemets autour de la valeur
		Value:      strings.Trim(rawValue, `"`),
		TTLSeconds: ttlSeconds,
	}, nil
}

// Parse une recherche comme : GET WHERE value > 18.
func parseWhereCommand(parts []string) (Command, error) {
	if len(parts) < 5 {
		return Command{}, fmt.Errorf("GET WHERE needs a field, an operator and a value")
	}

	field := FilterField(strings.ToLower(parts[2]))
	if field != FilterKey && field != FilterValue {
		return Command{}, fmt.Errorf("field must be key or value")
	}

	operator := FilterOperator(strings.ToLower(parts[3]))
	if !isFilterOperator(operator) {
		return Command{}, fmt.Errorf("unknown filter operator")
	}

	filterValue := strings.Trim(strings.Join(parts[4:], " "), `"`)
	if filterValue == "" {
		return Command{}, fmt.Errorf("missing filter value")
	}

	return Command{
		Type:        CommandWhere,
		FilterField: field,
		Operator:    operator,
		FilterValue: filterValue,
	}, nil
}

func isFilterOperator(operator FilterOperator) bool {
	switch operator {
	case OperatorEquals, OperatorContains, OperatorGreaterThan,
		OperatorGreaterOrEqual, OperatorLessThan, OperatorLessOrEqual:
		return true
	default:
		return false
	}
}

// Parse la commande ALL
func parseAllCommand(parts []string) (Command, error) {

	// ALL ne doit avoir aucun argument supplémentaire
	if len(parts) > 1 {
		return Command{}, fmt.Errorf("too many arguments")
	}

	// Retourne simplement une commande de type ALL
	return Command{Type: CommandAll}, nil
}
