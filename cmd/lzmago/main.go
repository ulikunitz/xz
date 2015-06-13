package main

//go:generate xb cat -o licenses.go xzLicense:github.com/uli-go/xz/LICENSE
//go:generate xb version-file -o version.go

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/uli-go/xz/gflag"
	"github.com/uli-go/xz/term"
)

const (
	lzmaExt  = ".lzma"
	usageStr = `Usage: lzmago [OPTION]... [FILE]...
Compress or uncompress FILEs in the .lzma format (by default, compress FILES
in place).

  -c, --stdout      write to standard output and don't delete input files
  -d, --decompress  force decompression
  -f, --force       force overwrite of output file and compress links
  -h, --help        give this help
  -k, --keep        keep (don't delete) input files
  -L, --license     display software license
  -q, --quiet       suppress all warnings
  -v, --verbose     verbose mode
  -V, --version     display version string
  -z, --compress    force compression
  -0 ... -9         compression preset; default is 6

With no file, or when FILE is -, read standard input.

Report bugs using <https://github.com/uli-go/xz/issues>.
`
)

func usage(w io.Writer) {
	fmt.Fprint(w, usageStr)
}

func licenses(w io.Writer) {
	out := `
github.com/uli-go/xz -- xz for Go 
=================================

{{.xz}}
`
	out = strings.TrimLeft(out, " \n")
	tmpl, err := template.New("licenses").Parse(out)
	if err != nil {
		log.Panicf("error %s parsing licenses template", err)
	}
	lmap := map[string]string{
		"xz": strings.TrimSpace(xzLicense),
	}
	if err = tmpl.Execute(w, lmap); err != nil {
		log.Fatalf("error %s writing licenses template", err)
	}
}

func main() {
	// setup logger
	cmdName := filepath.Base(os.Args[0])
	log.SetPrefix(fmt.Sprintf("%s: ", cmdName))
	log.SetFlags(0)

	// initialize flags
	gflag.CommandLine = gflag.NewFlagSet(cmdName, gflag.ExitOnError)
	gflag.Usage = func() { usage(os.Stderr); os.Exit(1) }
	var (
		help        = gflag.BoolP("help", "h", false, "")
		stdout      = gflag.BoolP("stdout", "c", false, "")
		decompress  = gflag.BoolP("decompress", "d", false, "")
		force       = gflag.BoolP("force", "f", false, "")
		keep        = gflag.BoolP("keep", "k", false, "")
		license     = gflag.BoolP("license", "L", false, "")
		versionFlag = gflag.BoolP("version", "V", false, "")
		quiet       = gflag.CounterP("quiet", "q", 0, "")
		verbose     = gflag.CounterP("verbose", "v", 0, "")
		preset      = gflag.Preset(0, 9, 6, "")
	)

	// process arguments
	gflag.Parse()

	if *help {
		usage(os.Stdout)
		os.Exit(0)
	}
	if *license {
		licenses(os.Stdout)
		os.Exit(0)
	}
	if *versionFlag {
		log.Printf("version %s\n", version)
		os.Exit(0)
	}
	var args []string
	if gflag.NArg() == 0 {
		if !*stdout {
			if *quiet > 0 {
				os.Exit(1)
			}
			log.Fatal("For help, type lzmago -h.")
		}
		args = []string{"-"}
	} else {
		args = gflag.Args()
	}

	if *stdout && !*decompress && !*force && term.IsTerminal(os.Stdout.Fd()) {
		if *quiet > 0 {
			os.Exit(1)
		}
		log.Print("Compressed data will not be written to a terminal.")
		log.SetPrefix("")
		log.Fatal("Use -f to force compression." +
			" For help type lzmago -h.")
	}

	log.Printf("decompress %t", *decompress)
	log.Printf("force %t", *force)
	log.Printf("keep %t", *keep)
	log.Printf("preset %d", *preset)
	log.Printf("stdout %t", *stdout)
	log.Printf("quiet %d", *quiet)
	log.Printf("verbose %d", *verbose)
	log.Printf("args %v", args)
}
