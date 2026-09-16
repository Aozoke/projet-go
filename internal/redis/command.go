package redis

// CommandType donne un nom Go aux commandes texte reconnues par le moteur.
type CommandType string

// SET ecrit, GET lit, GET_WHERE filtre, DELETE supprime et ALL liste les entrees.
const (
	CommandSet    CommandType = "SET"
	CommandGet    CommandType = "GET"
	CommandWhere  CommandType = "GET_WHERE"
	CommandDelete CommandType = "DELETE"
	CommandAll    CommandType = "ALL"
)

// FilterField indique ou chercher : dans les cles, les valeurs ou une cle du schema.
type FilterField string

const (
	FilterKey   FilterField = "key"
	FilterValue FilterField = "value"
)

// FilterOperator indique comment comparer : egalite, texte contenu ou comparaison de nombres.
type FilterOperator string

const (
	OperatorEquals         FilterOperator = "equals"
	OperatorContains       FilterOperator = "contains"
	OperatorGreaterThan    FilterOperator = ">"
	OperatorGreaterOrEqual FilterOperator = ">="
	OperatorLessThan       FilterOperator = "<"
	OperatorLessOrEqual    FilterOperator = "<="
)

// Command est le resultat du parser : le texte est range dans des champs nommes.
type Command struct {
	// Type choisit l'action ; Key et Value contiennent la cle et la valeur.
	Type  CommandType
	Key   string
	Value string
	// TTLSeconds est une duree en secondes ; 0 laisse le moteur utiliser son defaut.
	TTLSeconds int64
	// Ces trois champs servent seulement aux recherches GET WHERE.
	FilterField FilterField
	Operator    FilterOperator
	FilterValue string
}

// Entry contient la cle et la valeur a afficher, sans la date d'expiration.
type Entry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Operation est une ecriture a enregistrer dans le journal AOF pour pouvoir la rejouer.
type Operation struct {
	Type      CommandType `json:"type"`
	Key       string      `json:"key"`
	Value     string      `json:"value,omitempty"`
	ExpiresAt int64       `json:"expiresAt,omitempty"`
}

// StoredValue est la valeur réellement gardée en RAM et dans le snapshot.
// ExpiresAt vaut 0 quand la clé n'a pas de TTL.
// Sinon, c'est une date en millisecondes depuis le 1er janvier 1970, pas une duree.
type StoredValue struct {
	Value     string `json:"value"`
	ExpiresAt int64  `json:"expiresAt,omitempty"`
}

// Snapshot associe chaque cle a sa valeur et a sa date d'expiration.
type Snapshot map[string]StoredValue

// Result est la reponse d'une commande : reussite, valeur/liste ou message d'erreur.
type Result struct {
	OK      bool    `json:"ok"`
	Value   string  `json:"value,omitempty"`
	Entries []Entry `json:"entries,omitempty"`
	Error   string  `json:"error,omitempty"`
}

// BatchResult regroupe les reponses d'un lot, dans le meme ordre que les commandes.
// Writes contient les ecritures produites ; BufferSize compte celles en attente.
type BatchResult struct {
	Results    []Result    `json:"results"`
	Writes     []Operation `json:"writes"`
	BufferSize int         `json:"bufferSize"`
}
