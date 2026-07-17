package main

// moteur Redis lite
type Engine struct {
	state map[string]string
}

//state mémoire du moteur
//map[string]string : cle string -> valeur string

// crée un moteur prêt à être utilisé * engine on renvoie un pointeur
func NewEngine() *Engine {
	//& donne l’adresse du moteur.
	return &Engine{
		state: map[string]string{},
	}
}

//NewEngine crée un moteur prêt à l’emploi.
// Il contient un state,
//  c’est-à-dire une map en mémoire où les clés et les valeurs sont des strings.
//  Le *Engine indique qu’on renvoie un pointeur vers ce moteur,
//  pour travailler sur le même moteur ensuite.
