// Le parser transforme du texte en Command. Il ne lit ni ne modifie la base.

package redis

import (
	"fmt"
	"strconv"

	"strings"
)

// ReadCommandName lit le premier mot de la commande.
// Exemple : "get name" devient "GET".
func ReadCommandName(input string) string {

	trimmed := strings.TrimSpace(input)

	// Fields separe les mots sur les espaces : il ne conserve pas les espaces eux-memes.
	parts := strings.Fields(trimmed)

	if len(parts) == 0 {
		return ""
	}

	// ToUpper le met en majuscules pour reconnaitre aussi "get" ou "Get".
	return strings.ToUpper(parts[0])
}

// ParseCommand transforme une commande texte en structure Command.
// Si la commande est invalide, une erreur est renvoyée.
func ParseCommand(input string) (Command, error) {

	parts := strings.Fields(strings.TrimSpace(input))

	commandName := ReadCommandName(input)

	switch CommandType(commandName) {

	// GET accepte soit une cle, soit la forme GET WHERE.
	case CommandGet:
		// EqualFold compare WHERE sans tenir compte des majuscules/minuscules.
		if len(parts) > 1 && strings.EqualFold(parts[1], "WHERE") {
			return parseWhereCommand(parts)
		}
		return parseKeyCommand(CommandGet, parts)

	// DELETE attend une cle, comme le GET simple, mais n'accepte pas WHERE.
	case CommandDelete:
		return parseKeyCommand(CommandDelete, parts)

	case CommandSet:
		return parseSetCommand(parts)

	case CommandAll:
		return parseAllCommand(parts)

	default:
		return Command{}, fmt.Errorf("unknown command")
	}
}

// parseKeyCommand analyse les commandes qui prennent seulement une clé.
// Utilisé pour GET et DELETE.
func parseKeyCommand(commandType CommandType, parts []string) (Command, error) {

	if len(parts) < 2 {
		return Command{}, fmt.Errorf("missing key")
	}

	if len(parts) > 2 {
		return Command{}, fmt.Errorf("too many arguments")
	}

	return Command{
		Type: commandType,
		Key:  parts[1],
	}, nil
}

// parseSetCommand recupere la cle, la valeur entre guillemets et le TTL facultatif.
// Exemple : SET session "active" EX 60 donne une duree de vie de 60 secondes.
func parseSetCommand(parts []string) (Command, error) {

	if len(parts) < 2 {
		return Command{}, fmt.Errorf("missing key")
	}

	if len(parts) < 3 {
		return Command{}, fmt.Errorf("missing value")
	}

	// Par defaut, toute la fin de la commande appartient a la valeur, sans TTL explicite.
	valueEnd := len(parts)
	ttlSeconds := int64(0)

	// EX suivi d'un nombre ajoute une durée de vie à la clé.
	if len(parts) >= 5 && strings.EqualFold(parts[len(parts)-2], "EX") {
		parsedTTL, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
		if err != nil || parsedTTL <= 0 {
			return Command{}, fmt.Errorf("TTL must be a positive number")
		}

		ttlSeconds = parsedTTL
		// On exclut les deux derniers mots (EX et le nombre) de la valeur.
		valueEnd -= 2
	}

	// On regroupe la valeur sans la cle ni EX ; les espaces multiples deviennent un seul espace.
	rawValue := strings.Join(parts[2:valueEnd], " ")

	if !strings.HasPrefix(rawValue, `"`) || !strings.HasSuffix(rawValue, `"`) {
		return Command{}, fmt.Errorf("value must be quoted")
	}

	// Unquote enleve les guillemets exterieurs et decode les caracteres echappes,
	// par exemple \" devient un guillemet dans la valeur. Un texte invalide est refuse.
	value, err := strconv.Unquote(rawValue)
	if err != nil {
		return Command{}, fmt.Errorf("invalid quoted value")
	}

	return Command{
		Type: CommandSet,
		Key:  parts[1],

		Value:      value,
		TTLSeconds: ttlSeconds,
	}, nil
}

// parseWhereCommand analyse une recherche comme GET WHERE value > 18.
// Il prepare le filtre ; c'est le moteur qui cherchera les entrees correspondantes.
func parseWhereCommand(parts []string) (Command, error) {
	// Il faut au moins GET, WHERE, le champ, l'operateur et la valeur cherchee.
	if len(parts) < 5 {
		return Command{}, fmt.Errorf("GET WHERE needs a field, an operator and a value")
	}

	// Le champ peut etre key, value ou une cle du schema, comme age.
	field := FilterField(parts[2])

	// ToLower accepte aussi CONTAINS ; isFilterOperator refuse les operateurs inconnus.
	operator := FilterOperator(strings.ToLower(parts[3]))
	if !isFilterOperator(operator) {
		return Command{}, fmt.Errorf("unknown filter operator")
	}

	// La valeur cherchee peut contenir plusieurs mots.
	filterValue, err := parseFilterValue(strings.Join(parts[4:], " "))
	if err != nil {
		return Command{}, err
	}
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

// parseFilterValue accepte du texte brut (18) ou entre guillemets ("hello world").
// Si un guillemet ouvre la valeur, Unquote verifie et decode toute la chaine.
func parseFilterValue(rawValue string) (string, error) {
	if !strings.HasPrefix(rawValue, `"`) {
		return rawValue, nil
	}

	value, err := strconv.Unquote(rawValue)
	if err != nil {
		return "", fmt.Errorf("invalid quoted filter value")
	}
	return value, nil
}

// isFilterOperator renvoie true seulement pour les six comparaisons prises en charge.
func isFilterOperator(operator FilterOperator) bool {
	switch operator {
	case OperatorEquals, OperatorContains, OperatorGreaterThan,
		OperatorGreaterOrEqual, OperatorLessThan, OperatorLessOrEqual:
		return true
	default:
		return false
	}
}

// parseAllCommand accepte ALL tout seul, la commande qui demandera toutes les entrees.
func parseAllCommand(parts []string) (Command, error) {

	if len(parts) > 1 {
		return Command{}, fmt.Errorf("too many arguments")
	}

	return Command{Type: CommandAll}, nil
}
