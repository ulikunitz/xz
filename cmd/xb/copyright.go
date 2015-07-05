package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
)

const crUsageString = `xb copyright [options] <path>....

The xb copyright command adds a copyright remark to all go files below path.

  -h  prints this message and exits
`

func crUsage(w io.Writer) {
	fmt.Fprint(w, crUsageString)
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
		log.Printf("handle %s", path)
	}
}
