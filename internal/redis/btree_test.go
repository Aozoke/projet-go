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
