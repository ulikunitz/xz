package xz

import (
	"os"
	"testing"
)

func TestReadStreamHeader(t *testing.T) {
	xzfile, err := os.Open("LICENSE.xz")
	if err != nil {
		t.Fatal(err)
	}
	defer xzfile.Close()
	sf, err := readStreamHeader(xzfile)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("stream flags %s", sf)
}
