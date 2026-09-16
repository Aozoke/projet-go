package redis

import "github.com/samber/lo"

// BTreeItem associe une valeur numérique à la clé Redis qui la possède.
type BTreeItem struct {
	Value float64
	Key   string
}

// btreeNode est un noeud de l'arbre : plusieurs elements tries et, parfois, des enfants.
type btreeNode struct {
	leaf     bool         // true pour une feuille, c'est-a-dire un noeud sans enfants.
	items    []BTreeItem  // Les nombres et leurs cles, ranges du plus petit au plus grand.
	children []*btreeNode // Les branches entre les elements.
}

// BTree est un index ordonné. degree contrôle la taille de ses nœuds.
// On modifie les noeuds sur place pour ne pas recopier tout l'arbre a chaque insertion.
type BTree struct {
	degree int
	root   *btreeNode
}

// NewBTree cree un arbre vide avec une seule feuille comme racine (le premier noeud).
// Le degre vaut au moins 2 ; un noeud peut contenir au maximum 2*degree-1 elements.
func NewBTree(degree int) *BTree {
	if degree < 2 {
		degree = 2
	}

	return &BTree{
		degree: degree,
		root:   &btreeNode{leaf: true},
	}
}

// Insert ajoute un nombre et sa cle en gardant les elements tries dans l'arbre.
func (tree *BTree) Insert(item BTreeItem) {
	// Si la racine est pleine, on la divise sous une nouvelle racine : l'arbre grandit.
	if len(tree.root.items) == 2*tree.degree-1 {
		newRoot := &btreeNode{children: []*btreeNode{tree.root}}
		tree.splitChild(newRoot, 0)
		tree.root = newRoot
	}

	tree.insertNonFull(tree.root, item)
}

// insertNonFull descend vers la bonne feuille en evitant d'entrer dans un noeud plein.
func (tree *BTree) insertNonFull(node *btreeNode, item BTreeItem) {
	// On recule depuis la fin jusqu'a trouver la place de l'element dans l'ordre.
	position := len(node.items)
	for position > 0 && itemLess(item, node.items[position-1]) {
		position--
	}

	if node.leaf {
		// On decale les elements de la feuille pour inserer le nombre a sa place.
		node.items = append(node.items, BTreeItem{})
		copy(node.items[position+1:], node.items[position:])
		node.items[position] = item
		return
	}

	// Avant de descendre, on partage l'enfant plein et on choisit la bonne moitie.
	if len(node.children[position].items) == 2*tree.degree-1 {
		tree.splitChild(node, position)
		if itemLess(node.items[position], item) {
			position++
		}
	}

	tree.insertNonFull(node.children[position], item)
}

// splitChild coupe un enfant plein en deux et remonte son element du milieu dans le parent.
// Les petits elements restent a gauche ; les grands passent dans le nouveau noeud a droite.
func (tree *BTree) splitChild(parent *btreeNode, childIndex int) {
	child := parent.children[childIndex]
	middle := child.items[tree.degree-1]
	// La moitie droite est copiee pour que les deux noeuds aient leurs propres elements.
	right := &btreeNode{
		leaf:  child.leaf,
		items: append([]BTreeItem(nil), child.items[tree.degree:]...),
	}

	child.items = child.items[:tree.degree-1]
	if !child.leaf {
		// Un noeud interne a aussi des enfants : on les partage entre les deux moities.
		right.children = append([]*btreeNode(nil), child.children[tree.degree:]...)
		child.children = child.children[:tree.degree]
	}

	// On fait une place dans le parent pour l'element du milieu.
	parent.items = append(parent.items, BTreeItem{})
	copy(parent.items[childIndex+1:], parent.items[childIndex:])
	parent.items[childIndex] = middle

	// On rattache le nouveau noeud juste apres l'enfant gauche dans le parent.
	parent.children = append(parent.children, nil)
	copy(parent.children[childIndex+2:], parent.children[childIndex+1:])
	parent.children[childIndex+1] = right
}

// RangeItems recupere les nombres qui respectent une limite (<, <=, > ou >=).
// Il part du bon cote et s'arrete au premier nombre hors limite.
// Une plage qui correspond a presque toute la base visitera tout de meme beaucoup d'elements.
func (tree *BTree) RangeItems(operator FilterOperator, target float64) []BTreeItem {
	items := make([]BTreeItem, 0)

	if operator == OperatorLessThan || operator == OperatorLessOrEqual {
		// Pour < ou <=, on commence au plus petit ; false demande d'arreter le parcours.
		tree.walkAscending(tree.root, func(item BTreeItem) bool {
			if !matchesRange(item.Value, operator, target) {
				return false
			}
			items = append(items, item)
			return true
		})
		return items
	}

	// Pour > ou >=, on part du plus grand et on descend jusqu'a la limite.
	tree.walkDescending(tree.root, func(item BTreeItem) bool {
		if !matchesRange(item.Value, operator, target) {
			return false
		}
		items = append(items, item)
		return true
	})

	// Les resultats viennent du plus grand au plus petit : on inverse leur ordre.
	// On echange les deux extremites, puis on se rapproche du milieu.
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
	return items
}

// Range garde uniquement les cles des resultats de RangeItems.
func (tree *BTree) Range(operator FilterOperator, target float64) []string {
	return lo.Map(tree.RangeItems(operator, target), func(item BTreeItem, _ int) string {
		return item.Key
	})
}

// walkAscending visite les nombres du plus petit au plus grand.
// visit est une fonction recue en argument : son false arrete aussi les appels parents.
func (tree *BTree) walkAscending(node *btreeNode, visit func(BTreeItem) bool) bool {
	for index, item := range node.items {
		// L'enfant a gauche contient des nombres a visiter avant l'element courant.
		if !node.leaf && !tree.walkAscending(node.children[index], visit) {
			return false
		}
		if !visit(item) {
			return false
		}
	}

	// Apres le dernier element, il reste l'enfant tout a droite, sauf pour une feuille.
	return node.leaf || tree.walkAscending(node.children[len(node.items)], visit)
}

// walkDescending fait le meme parcours a l'envers : enfant droit, element, puis gauche.
// Il transmet lui aussi l'ordre d'arret (false) des qu'une limite est atteinte.
func (tree *BTree) walkDescending(node *btreeNode, visit func(BTreeItem) bool) bool {
	for index := len(node.items) - 1; index >= 0; index-- {
		if !node.leaf && !tree.walkDescending(node.children[index+1], visit) {
			return false
		}
		if !visit(node.items[index]) {
			return false
		}
	}

	return node.leaf || tree.walkDescending(node.children[0], visit)
}

// itemLess dit si left doit etre place avant right.
// A nombre egal, la cle departage les elements pour donner un ordre stable.
func itemLess(left BTreeItem, right BTreeItem) bool {
	if left.Value == right.Value {
		return left.Key < right.Key
	}

	return left.Value < right.Value
}

// matchesRange applique la comparaison demandee entre un nombre et la limite target.
// Un operateur qui n'est pas une comparaison de plage renvoie false.
func matchesRange(value float64, operator FilterOperator, target float64) bool {
	switch operator {
	case OperatorGreaterThan:
		return value > target
	case OperatorGreaterOrEqual:
		return value >= target
	case OperatorLessThan:
		return value < target
	case OperatorLessOrEqual:
		return value <= target
	default:
		return false
	}
}
