package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const crUsageString = `xb copyright [options] <path>....

The xb copyright command adds a copyright remark to all go files below path.

  -h  prints this message and exits
`

func crUsage(w io.Writer) {
	fmt.Fprint(w, crUsageString)
}

func addCopyright(path string) error {
	fmt.Println(path)
	return nil
}

func walkCopyrights(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}
	if !strings.HasSuffix(info.Name(), ".go") {
		return nil
	}
	return addCopyright(path)
}

func copyright() {
	cmdName := os.Args[0]
	log.SetPrefix(fmt.Sprintf("%s: ", cmdName))
	log.SetFlags(0)

	flag.CommandLine = flag.NewFlagSet(cmdName, flag.ExitOnError)
	flag.Usage = func() { crUsage(os.Stderr); os.Exit(1) }

	help := flag.Bool("h", false, "")

	flag.Parse()

	if *help {
		crUsage(os.Stdout)
		os.Exit(0)
	}

	for _, path := range flag.Args() {
		fi, err := os.Stat(path)
		if err != nil {
			log.Print(err)
			continue
		}
		if !fi.IsDir() {
			log.Printf("%s is not a directory", path)
			continue
		}
		if err = filepath.Walk(path, walkCopyrights); err != nil {
			log.Fatalf("%s error %s", path, err)
		}
	}
}
