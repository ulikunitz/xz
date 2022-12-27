package xz_test

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/ulikunitz/xz"
)

func ExampleReader() {
	const file = "testdata/fox.xz"
	f, err := os.Open(file)
	if err != nil {
		log.Fatalf("os.Open(%q) error %s", file, err)
	}
	defer f.Close()
	r, err := xz.NewReader(bufio.NewReader(f))
	if err != nil {
		log.Fatalf("xz.NewReasder error %s", err)
	}
	if _, err = io.Copy(os.Stdout, r); err != nil {
		log.Fatalf("io.Copy error %s", err)
	}
	// Output:
	// The quick brown fox jumps over the lazy dog.
}

func ExampleWriter() {
	const file = "testdata/example.xz"
	f, err := os.Create(file)
	if err != nil {
		log.Fatalf("os.Open(%q) error %s", file, err)
	}
	defer f.Close()
	w, err := xz.NewWriter(f)
	if err != nil {
		log.Fatalf("xz.NewWriter(f) error %s", err)
	}
	defer w.Close()
	_, err = fmt.Fprintln(w, "The brown fox jumps over the lazy dog.")
	if err != nil {
		log.Fatalf("fmt.Fprintln error %s", err)
	}
	if err = w.Close(); err != nil {
		log.Fatalf("w.Close() error %s", err)
	}
	// Output:
}
