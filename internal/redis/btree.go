package redis

// BTreeItem associe une valeur numérique à la clé Redis qui la possède.
type BTreeItem struct {
	Value float64
	Key   string
}

type btreeNode struct {
	leaf     bool
	items    []BTreeItem
	children []*btreeNode
}

// BTree est un index ordonné. degree contrôle la taille de ses nœuds.
type BTree struct {
	degree int
	root   *btreeNode
}

func NewBTree(degree int) *BTree {
	if degree < 2 {
		degree = 2
	}

	return &BTree{
		degree: degree,
		root:   &btreeNode{leaf: true},
	}
}

func (tree *BTree) Insert(item BTreeItem) {
	if len(tree.root.items) == 2*tree.degree-1 {
		newRoot := &btreeNode{children: []*btreeNode{tree.root}}
		tree.splitChild(newRoot, 0)
		tree.root = newRoot
	}

	tree.insertNonFull(tree.root, item)
}

func (tree *BTree) insertNonFull(node *btreeNode, item BTreeItem) {
	position := len(node.items)
	for position > 0 && itemLess(item, node.items[position-1]) {
		position--
	}

	if node.leaf {
		node.items = append(node.items, BTreeItem{})
		copy(node.items[position+1:], node.items[position:])
		node.items[position] = item
		return
	}

	if len(node.children[position].items) == 2*tree.degree-1 {
		tree.splitChild(node, position)
		if itemLess(node.items[position], item) {
			position++
		}
	}

	tree.insertNonFull(node.children[position], item)
}

func (tree *BTree) splitChild(parent *btreeNode, childIndex int) {
	child := parent.children[childIndex]
	middle := child.items[tree.degree-1]
	right := &btreeNode{
		leaf:  child.leaf,
		items: append([]BTreeItem(nil), child.items[tree.degree:]...),
	}

	child.items = child.items[:tree.degree-1]
	if !child.leaf {
		right.children = append([]*btreeNode(nil), child.children[tree.degree:]...)
		child.children = child.children[:tree.degree]
	}

	parent.items = append(parent.items, BTreeItem{})
	copy(parent.items[childIndex+1:], parent.items[childIndex:])
	parent.items[childIndex] = middle

	parent.children = append(parent.children, nil)
	copy(parent.children[childIndex+2:], parent.children[childIndex+1:])
	parent.children[childIndex+1] = right
}

// Range renvoie les clés dans l'ordre de leur valeur numérique.
func (tree *BTree) Range(operator FilterOperator, target float64) []string {
	items := tree.itemsInOrder(tree.root, nil)
	keys := make([]string, 0)

	for _, item := range items {
		if matchesRange(item.Value, operator, target) {
			keys = append(keys, item.Key)
		}
	}

	return keys
}

func (tree *BTree) itemsInOrder(node *btreeNode, items []BTreeItem) []BTreeItem {
	for index, item := range node.items {
		if !node.leaf {
			items = tree.itemsInOrder(node.children[index], items)
		}
		items = append(items, item)
	}

	if !node.leaf {
		items = tree.itemsInOrder(node.children[len(node.items)], items)
	}

	return items
}

func itemLess(left BTreeItem, right BTreeItem) bool {
	if left.Value == right.Value {
		return left.Key < right.Key
	}

	return left.Value < right.Value
}

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
