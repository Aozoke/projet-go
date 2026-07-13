package main

import "strings"

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

}
