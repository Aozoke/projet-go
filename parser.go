package main

import (
	"fmt"
	"strings"
)

// La première fonction s’appelle ReadCommandName.
// Elle sert juste à récupérer le premier mot de la commande.

// on crée une fonction appelé ReadCommandName qui reçoit string et renvois string ( fun nom paramatre type, type de retour)
func ReadCommandName(input string) string {
	trimmed := strings.TrimSpace(input)
	//Supprime les espaces inutiles au début et à la fin.
	parts := strings.Fields(trimmed)
	//Découpe le texte en morceaux séparés par les espaces.

	if len(parts) == 0 {
		return ""
	}
	//Si la liste est vide, ça veut dire que l’utilisateur n’a rien écrit.Dans ce cas, on renvoie une string vide.

	return strings.ToUpper(parts[0])
	//transforme une string en majuscules.
	//return parts[0]
	/*
		Sinon, on renvoie le premier morceau.
		Donc "GET" pour "GET name".
	*/

}

/*
Command : notre struct de commande
error : une erreur si la commande est invalide
Command{} : une commande vide
nil : aucune erreur

ourquoi CommandType(...) ?
Parce que commandName est une string, alors que Type attend un CommandType

	elle prend toute la commande texte et essaie de construire une vraie commande Go.
*/
func ParseCommand(input string) (Command, error) {
	//On nettoie les espaces, puis on découpe.
	parts := strings.Fields(strings.TrimSpace(input))
	//On récupère le premier mot de la commande
	commandName := ReadCommandName(input)

	/*
		ParseCommand ne sait parser que GET.
		Donc si la commande n’est pas GET, on refuse.
	*/
	// if commandName != string(CommandGet) {
	// 	return Command{}, fmt.Errorf("unknown command")
	// 	//Si la commande n’est pas GET, je renvoie une erreur.
	// }
	//on le remplace par :

	//si la commande n’est pas GET ET n’est pas DELETE.
	// if commandName != string(CommandGet) && commandName != string(CommandDelete) {
	// 	return Command{}, fmt.Errorf("unknown command")
	// }

	if commandName != string(CommandGet) && commandName != string(CommandDelete) && commandName != string(CommandSet) {
		return Command{}, fmt.Errorf("unknown command")
	}

	if len(parts) < 2 {
		// est ce que la list parts contient moins de  2 elements
		return Command{}, fmt.Errorf("missing key")
	}

	value := ""
	if commandName == string(CommandSet) {
		if len(parts) < 3 {
			return Command{}, fmt.Errorf("missing value")
		}

		/*
			strings.Trim enlève certains caractères au début et à la fin d’une string.
		*/
		value = strings.Trim(parts[2], `"`)
	}

	return Command{
		Type:  CommandType(commandName),
		Key:   parts[1],
		Value: value,
	}, nil
}
