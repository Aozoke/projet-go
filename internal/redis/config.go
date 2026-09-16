package redis

import "time"

const (
	// Valeur de secours si aucun degre valide n'est fourni au moteur.
	DefaultBTreeDegree = 8
)

// Config regroupe les reglages du moteur. Le chargement du .env se fait ailleurs.
type Config struct {
	// Duree de vie par defaut en secondes ; 0 signifie sans expiration.
	DefaultTTLSeconds int64
	// Regle la taille des noeuds du B-Tree : au maximum 2*degre-1 elements par noeud.
	BTreeDegree int
	// Fonction qui donne l'heure. Les tests la remplacent pour simuler le temps.
	Now func() time.Time
}

// defaultConfig prepare les reglages de depart, avec l'heure reelle de la machine.
// Sans configuration, les cles n'expirent pas.
func defaultConfig() Config {
	return Config{
		BTreeDegree: DefaultBTreeDegree,
		Now:         time.Now,
	}
}
