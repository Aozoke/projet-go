package main

import (
	"strings"
)

// on crée une fonction appelé ReadCommandName qui reçoit string et renvois string
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
*/
func ParseCommand(input string) (Command, error) {
	//On nettoie les espaces, puis on découpe.
	parts := strings.Fields(strings.TrimSpace(input))
	//On récupère le premier mot de la commande.
	commandName := ReadCommandName(input)

	return Command{
		Type: CommandType(commandName),
		Key:  parts[1],
	}, nil
}
