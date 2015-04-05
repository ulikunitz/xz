package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/ogier/pflag"
	"github.com/uli-go/xz/old/lzma"
)

const (
	cmdName = "lzmago"
	lzmaExt = ".lzma"
)

var (
	uncompress = pflag.BoolP("decompress", "d", false, "decompresses files")
)

func compressedName(name string) (string, error) {
	if filepath.Ext(name) == lzmaExt {
		return "", fmt.Errorf(
			"%s already has %s extension -- unchanged",
			name, lzmaExt)
	}
	return name + lzmaExt, nil
}

func cleanup(r, w *os.File, ferr error) error {
	var cerr error
	if r != nil {
		if err := r.Close(); err != nil {
			cerr = err
		}
		if ferr == nil {
			err := os.Remove(r.Name())
			if cerr == nil && err != nil {
				cerr = err
			}
		}
	}
	if w != nil {
		if err := w.Close(); cerr == nil && err != nil {
			cerr = err
		}
		if ferr != nil {
			err := os.Remove(w.Name())
			if cerr == nil && err != nil {
				cerr = err
			}
		}
	}
	if ferr == nil && cerr != nil {
		ferr = cerr
	}
	return ferr
}

func compressFile(name string) (err error) {
	var r, w *os.File
	defer func() {
		err = cleanup(r, w, err)
	}()
	compName, err := compressedName(name)
	if err != nil {
		return
	}
	r, err = os.Open(name)
	if err != nil {
		return
	}
	w, err = os.Create(compName)
	if err != nil {
		return
	}
	bw := bufio.NewWriter(w)
	lw, err := lzma.NewWriter(bw)
	if err != nil {
		return
	}
	if _, err = io.Copy(lw, r); err != nil {
		return
	}
	if err = lw.Close(); err != nil {
		return
	}
	err = bw.Flush()
	return
}

func uncompressedName(name string) (uname string, err error) {
	ext := filepath.Ext(name)
	if ext != lzmaExt {
		return "", fmt.Errorf(
			"%s: file extension %s unknown -- ignored", name, ext)
	}
	return name[:len(name)-len(ext)], nil
}

func uncompressFile(name string) (err error) {
	var r, w *os.File
	defer func() {
		err = cleanup(r, w, err)
	}()
	uname, err := uncompressedName(name)
	if err != nil {
		return
	}
	r, err = os.Open(name)
	if err != nil {
		return
	}
	lr, err := lzma.NewReader(bufio.NewReader(r))
	if err != nil {
		return
	}
	w, err = os.Create(uname)
	if err != nil {
		return
	}
	_, err = io.Copy(w, lr)
	return
}

func main() {
	log.SetPrefix(fmt.Sprintf("%s: ", cmdName))
	log.SetFlags(0)
	pflag.Parse()
	if len(pflag.Args()) == 0 {
		log.Print("For help use option -h")
		os.Exit(0)
	}
	if *uncompress {
		// uncompress files
		for _, name := range pflag.Args() {
			if err := uncompressFile(name); err != nil {
				log.Fatal(err)
			}
		}
	} else {
		// compress files
		for _, name := range pflag.Args() {
			if err := compressFile(name); err != nil {
				log.Fatal(err)
			}
		}
	}
}
