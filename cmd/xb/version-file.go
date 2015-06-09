package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

const vfUsage = `xb version-file [options] <id>:<path>...

This command puts the contents of the files given as relative paths to
the GOPATH variable as string constants into a go file. 

   -h  prints this message and exits
   -p  package name (default main)
   -o  file name of output

`

func versionFileUsage(w io.Writer) {
	fmt.Fprint(w, vfUsage)
}

func versionFile() {
	cmdName := filepath.Base(os.Args[0])
	log.SetPrefix(fmt.Sprintf("%s: ", cmdName))
	log.SetFlags(0)

	flag.CommandLine = flag.NewFlagSet(cmdName, flag.ExitOnError)
	flag.Usage = func() { versionFileUsage(os.Stderr); os.Exit(1) }

	help := flag.Bool("h", false, "")
	pkg := flag.String("p", "main", "")
	out := flag.String("o", "", "")

	flag.Parse()

	if *help {
		versionFileUsage(os.Stdout)
		os.Exit(0)
	}

	if *pkg == "" {
		log.Fatal("option -p must not be empty")
	}

	var err error
	w := os.Stdout
	if *out != "" {
		if w, err = os.Create(*out); err != nil {
			log.Fatal(err)
		}
	}

	b, err := exec.Command("git", "describe").Output()
	if err != nil {
		log.Fatalf("error %s while executing git describe", err)
	}
	version := strings.TrimSpace(string(b))

	versionTmpl := `package main

const version = "{{.}}"
`
	tmpl := template.Must(template.New("version").Parse(versionTmpl))
	if err = tmpl.Execute(w, version); err != nil {
		log.Fatal(err)
	}
}
