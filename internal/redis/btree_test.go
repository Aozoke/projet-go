package redis

import "testing"

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

func TestBTreeAllRangeOperators(t *testing.T) {
	tree := NewBTree(2)
	for _, item := range []BTreeItem{
		{Value: 10, Key: "ten"},
		{Value: 20, Key: "twenty"},
		{Value: 30, Key: "thirty"},
	} {
		tree.Insert(item)
	}

	tests := []struct {
		operator FilterOperator
		expected []string
	}{
		{OperatorGreaterThan, []string{"thirty"}},
		{OperatorGreaterOrEqual, []string{"twenty", "thirty"}},
		{OperatorLessThan, []string{"ten"}},
		{OperatorLessOrEqual, []string{"ten", "twenty"}},
	}

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
