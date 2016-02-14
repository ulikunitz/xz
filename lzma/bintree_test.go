package lzma

import (
	"io"
	"os"
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
	if err = bt.dump(os.Stdout); err != nil {
		t.Fatalf("bt.dump error %s", err)
	}
	tests := []string{"Sieg", "Sieb", "Simu"}
	for _, c := range tests {
		x := xval([]byte(c))
		v := bt.find(x)
		t.Logf("v for %q: %d", c, v)
	}
}
