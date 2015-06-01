package lzma

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"testing"

	"github.com/uli-go/xz/randtxt"
)

func TestWriterCycle(t *testing.T) {
	orig := readOrigFile(t)
	buf := new(bytes.Buffer)
	w, err := NewWriter(buf)
	if err != nil {
		t.Fatalf("NewWriter: error %s", err)
	}
	n, err := w.Write(orig)
	if err != nil {
		t.Fatalf("w.Write error %s", err)
	}
	if n != len(orig) {
		t.Fatalf("w.Write returned %d; want %d", n, len(orig))
	}
	if err = w.Close(); err != nil {
		t.Fatalf("w.Close error %s", err)
	}
	t.Logf("buf.Len() %d len(orig) %d", buf.Len(), len(orig))
	if buf.Len() > len(orig) {
		t.Errorf("buf.Len()=%d bigger then len(orig)=%d", buf.Len(),
			len(orig))
	}
	lr, err := NewReader(buf)
	if err != nil {
		t.Fatalf("NewReader error %s", err)
	}
	decoded, err := ioutil.ReadAll(lr)
	if err != nil {
		t.Fatalf("ReadAll(lr) error %s", err)
	}
	t.Logf("%s", decoded)
	if len(orig) != len(decoded) {
		t.Fatalf("length decoded is %d; want %d", len(decoded),
			len(orig))
	}
	if !bytes.Equal(orig, decoded) {
		t.Fatalf("decoded file differs from original")
	}
}

func TestWriterLongData(t *testing.T) {
	const (
		seed = 49
		size = 82237
	)
	r := io.LimitReader(randtxt.NewReader(rand.NewSource(seed)), size)
	txt, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll error %s", err)
	}
	if len(txt) != size {
		t.Fatalf("ReadAll read %d bytes; want %d", len(txt), size)
	}
	buf := &bytes.Buffer{}
	params := Default
	params.DictSize = 0x4000
	w, err := NewWriterParams(buf, params)
	if err != nil {
		t.Fatalf("NewWriter error %s", err)
	}
	n, err := w.Write(txt)
	if err != nil {
		t.Fatalf("w.Write error %s", err)
	}
	if n != len(txt) {
		t.Fatalf("w.Write wrote %d bytes; want %d", n, size)
	}
	if err = w.Close(); err != nil {
		t.Fatalf("w.Close error %s", err)
	}
	t.Logf("compressed length %d", buf.Len())
	lr, err := NewReader(buf)
	if err != nil {
		t.Fatalf("NewReader error %s", err)
	}
	txtRead, err := ioutil.ReadAll(lr)
	if err != nil {
		t.Fatalf("ReadAll(lr) error %s", err)
	}
	if len(txtRead) != size {
		t.Fatalf("ReadAll(lr) returned %d bytes; want %d",
			len(txtRead), size)
	}
	if !bytes.Equal(txtRead, txt) {
		t.Fatal("ReadAll(lr) returned txt differs from origin")
	}
}

// The example uses the buffered reader and writer from package bufio.
func Example_writer() {
	pr, pw := io.Pipe()
	go func() {
		bw := bufio.NewWriter(pw)
		w, err := NewWriter(bw)
		if err != nil {
			log.Fatal(err)
		}
		input := []byte("The quick brown fox jumps over the lazy dog.")
		if _, err = w.Write(input); err != nil {
			log.Fatal(err)
		}
		if err = w.Close(); err != nil {
			log.Fatal(err)
		}
		// reader waits for the data
		if err = bw.Flush(); err != nil {
			log.Fatal(err)
		}
	}()
	r, err := NewReader(pr)
	if err != nil {
		log.Fatal(err)
	}
	_, err = io.Copy(os.Stdout, r)
	if err != nil {
		log.Fatal(err)
	}
	// Output:
	// The quick brown fox jumps over the lazy dog.
}
