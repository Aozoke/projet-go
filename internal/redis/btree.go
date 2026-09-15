package redis

import "github.com/samber/lo"

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

// RangeItems parcourt l'arbre dans le bon sens et s'arrête dès que la limite
// est dépassée. Une plage ne transforme donc plus tout le B-Tree en slice.
func (tree *BTree) RangeItems(operator FilterOperator, target float64) []BTreeItem {
	items := make([]BTreeItem, 0)

	if operator == OperatorLessThan || operator == OperatorLessOrEqual {
		tree.walkAscending(tree.root, func(item BTreeItem) bool {
			if !matchesRange(item.Value, operator, target) {
				return false
			}
			items = append(items, item)
			return true
		})
		return items
	}

	tree.walkDescending(tree.root, func(item BTreeItem) bool {
		if !matchesRange(item.Value, operator, target) {
			return false
		}
		items = append(items, item)
		return true
	})

	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
	return items
}

func (tree *BTree) Range(operator FilterOperator, target float64) []string {
	return lo.Map(tree.RangeItems(operator, target), func(item BTreeItem, _ int) string {
		return item.Key
	})
}

func (tree *BTree) walkAscending(node *btreeNode, visit func(BTreeItem) bool) bool {
	for index, item := range node.items {
		if !node.leaf && !tree.walkAscending(node.children[index], visit) {
			return false
		}
		if !visit(item) {
			return false
		}
	}

	return node.leaf || tree.walkAscending(node.children[len(node.items)], visit)
}

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
