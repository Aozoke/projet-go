package redis

import "testing"

// TestBTreeRange insere des nombres dans le desordre et cherche ceux >= 20.
// Avec un degre de 2, le quatrieme ajout force aussi le partage d'une racine pleine.
func TestBTreeRange(t *testing.T) {
	tree := NewBTree(2)
	tree.Insert(BTreeItem{Value: 30, Key: "thirty"})
	tree.Insert(BTreeItem{Value: 10, Key: "ten"})
	tree.Insert(BTreeItem{Value: 20, Key: "twenty"})
	tree.Insert(BTreeItem{Value: 40, Key: "forty"})

	keys := tree.Range(OperatorGreaterOrEqual, 20)
	if len(keys) != 3 || keys[0] != "twenty" || keys[2] != "forty" {
		t.Fatalf("unexpected keys: %+v", keys)
	}
}

// TestBTreeAllRangeOperators compare les quatre operateurs autour de la limite 20.
func TestBTreeAllRangeOperators(t *testing.T) {
	tree := NewBTree(2)
	for _, item := range []BTreeItem{
		{Value: 10, Key: "ten"},
		{Value: 20, Key: "twenty"},
		{Value: 30, Key: "thirty"},
	} {
		tree.Insert(item)
	}

	// Une liste de cas rassemble l'operateur a essayer et les cles attendues.
	tests := []struct {
		operator FilterOperator
		expected []string
	}{
		{OperatorGreaterThan, []string{"thirty"}},
		{OperatorGreaterOrEqual, []string{"twenty", "thirty"}},
		{OperatorLessThan, []string{"ten"}},
		{OperatorLessOrEqual, []string{"ten", "twenty"}},
	}

	// Pour chaque cas, on verifie d'abord la taille, puis chaque cle dans l'ordre.
	for _, test := range tests {
		keys := tree.Range(test.operator, 20)
		if len(keys) != len(test.expected) {
			t.Fatalf("operator %s: expected %v, got %v", test.operator, test.expected, keys)
		}
		for index := range keys {
			if keys[index] != test.expected[index] {
				t.Fatalf("operator %s: expected %v, got %v", test.operator, test.expected, keys)
			}
		}
	}
}
