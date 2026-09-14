// tester directement les commandes dans le terminal
package main

import (
	// Permet de lire ce que l'utilisateur tape dans le terminal
	"bufio"
	"fmt"

	// Donne accès à l'entrée standard du terminal
	"os"
	// Permet de manipuler et nettoyer les chaînes de caractères
	"strings"
	// Importe notre moteur Redis
	"github.com/Aozoke/projet-go/internal/redis"
)

// Point d'entrée du programme CLI
func main() {

	// Crée une nouvelle instance du moteur Redis
	engine := redis.NewEngine()

	// La commande est saisie dans le terminal
	scanner := bufio.NewScanner(os.Stdin)

	// Affiche le nom du programme
	fmt.Println("WasmRedis CLI")

	// Affiche quelques exemples de commandes disponibles
	fmt.Println(`Exemples : SET name "matt" | SET session "active" EX 60 | GET WHERE value >= 18 | ALL | exit`)

	// Boucle principale : on attend des commandes tant que le programme tourne
	for {

		// Affiche le symbole avant chaque saisie utilisateur
		fmt.Print("> ")

		// Lit la prochaine ligne tapée dans le terminal
		if !scanner.Scan() {

			// Arrête la boucle s'il n'y a plus rien à lire
			break
		}

		//  scanner.Text() récupère le texte que tu viens de taper. et enleve les espace inutile
		line := strings.TrimSpace(scanner.Text())

		// Si l'utilisateur tape "exit", on quitte le programme
		if line == "exit" {
			break
		}

		// La commande est envoyée au moteur
		result, _ := engine.ExecuteText(line)

		// Vérifie si la commande a échoué
		if !result.OK {

			// Affiche le message d'erreur
			fmt.Println("error:", result.Error)

			// Revient au début de la boucle
			continue
		}

		// Vérifie si le résultat contient plusieurs entrées
		// C'est notamment le cas avec la commande ALL
		if len(result.Entries) > 0 {

			// Parcourt toutes les entrées retournées
			for _, entry := range result.Entries {

				// Affiche chaque clé avec sa valeur
				fmt.Printf("%s = %s\n", entry.Key, entry.Value)
			}

			// Revient au début de la boucle
			continue
		}

		// Vérifie si le résultat contient une valeur simple
		// C'est notamment le cas avec GET
		if result.Value != "" {

			// Affiche la valeur récupérée
			fmt.Println(result.Value)

			// Revient au début de la boucle
			continue
		}

		// Si aucune valeur particulière n'est retournée,
		// la commande s'est simplement bien exécutée
		fmt.Println("OK")
	}

	// Vérifie si le scanner a rencontré une erreur de lecture
	if err := scanner.Err(); err != nil {

		// Affiche l'erreur éventuelle
		fmt.Println("error:", err)
	}
}
