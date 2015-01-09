package lzma

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"os"
	"testing"
)

func TestWriterCycle(t *testing.T) {
	wdebug, err := os.Create("writer.txt")
	if err != nil {
		t.Fatalf("OpenFile writer.txt error %s", err)
	}
	defer wdebug.Close()
	debugOn(wdebug)
	defer debugOff()

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
	rdebug, err := os.Create("reader.txt")
	if err != nil {
		t.Fatalf("OpenFile reader.txt error %s", err)
	}
	defer rdebug.Close()
	debugOn(rdebug)
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

// The example uses the buffered reader and writer from package bufio.
func Example_writer() {
	pr, pw := io.Pipe()
	br, bw := bufio.NewReader(pr), bufio.NewWriter(pw)
	go func() {
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
	r, err := NewReader(br)
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
