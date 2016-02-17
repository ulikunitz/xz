package lzma

import (
	"io"
	"testing"
)

func TestBinTree_Find(t *testing.T) {
	bt, err := newBinTree(30)
	if err != nil {
		t.Fatal(err)
	}
	const s = "Klopp feiert mit Liverpool seinen hoechsten SiegSieg"
	n, err := io.WriteString(bt, s)
	if err != nil {
		t.Fatalf("WriteString error %s", err)
	}
	if n != len(s) {
		t.Fatalf("WriteString returned %d; want %d", n, len(s))
	}

	/* dump info writes the complete tree
	if err = bt.dump(os.Stdout); err != nil {
		t.Fatalf("bt.dump error %s", err)
	}
	*/

	tests := []string{"Sieg", "Sieb", "Simu"}
	for _, c := range tests {
		x := xval([]byte(c))
		a, b := bt.search(bt.root, x)
		t.Logf("%q: a, b == %d, %d", c, a, b)
	}
}

func TestBinTree_PredSucc(t *testing.T) {
	bt, err := newBinTree(30)
	if err != nil {
		t.Fatal(err)
	}
	const s = "Klopp feiert mit Liverpool seinen hoechsten Sieg."
	n, err := io.WriteString(bt, s)
	if err != nil {
		t.Fatalf("WriteString error %s", err)
	}
	if n != len(s) {
		t.Fatalf("WriteString returned %d; want %d", n, len(s))
	}
	for v := bt.min(bt.root); v != null; v = bt.succ(v) {
		t.Log(dumpX(bt.node[v].x))
	}
	t.Log("")
	for v := bt.max(bt.root); v != null; v = bt.pred(v) {
		t.Log(dumpX(bt.node[v].x))
	}
}
