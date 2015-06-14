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

type options struct {
	help       bool
	stdout     bool
	decompress bool
	force      bool
	keep       bool
	license    bool
	version    bool
	quiet      int
	verbose    int
	preset     int
}

func (o *options) Init() {
	if o.preset != 0 {
		log.Panicf("options are already initialized")
	}
	gflag.BoolVarP(&o.help, "help", "h", false, "")
	gflag.BoolVarP(&o.decompress, "decompress", "d", false, "")
	gflag.BoolVarP(&o.force, "force", "f", false, "")
	gflag.BoolVarP(&o.keep, "keep", "k", false, "")
	gflag.BoolVarP(&o.license, "license", "L", false, "")
	gflag.BoolVarP(&o.version, "version", "V", false, "")
	gflag.CounterVarP(&o.quiet, "quiet", "q", 0, "")
	gflag.CounterVarP(&o.verbose, "verbose", "v", 0, "")
	gflag.PresetVar(&o.preset, 0, 9, 6, "")
}

func main() {
	// setup logger
	cmdName := filepath.Base(os.Args[0])
	log.SetPrefix(fmt.Sprintf("%s: ", cmdName))
	log.SetFlags(0)

	// initialize flags
	gflag.CommandLine = gflag.NewFlagSet(cmdName, gflag.ExitOnError)
	gflag.Usage = func() { usage(os.Stderr); os.Exit(1) }
	opts := options{}
	opts.Init()
	gflag.Parse()

	if opts.help {
		usage(os.Stdout)
		os.Exit(0)
	}
	if opts.license {
		licenses(os.Stdout)
		os.Exit(0)
	}
	if opts.version {
		log.Printf("version %s\n", version)
		os.Exit(0)
	}
	var args []string
	if gflag.NArg() == 0 {
		if !opts.stdout {
			if opts.quiet > 0 {
				os.Exit(1)
			}
			log.Fatal("For help, type lzmago -h.")
		}
		args = []string{"-"}
	} else {
		args = gflag.Args()
	}

	if opts.stdout && !opts.decompress && !opts.force &&
		term.IsTerminal(os.Stdout.Fd()) {
		if opts.quiet > 0 {
			os.Exit(1)
		}
		log.Print("Compressed data will not be written to a terminal.")
		log.SetPrefix("")
		log.Fatal("Use -f to force compression." +
			" For help type lzmago -h.")
	}

	for _, arg := range args {
		f := arg
		if f == "-" {
			f = "stdin"
		}
		if opts.decompress {
			log.SetPrefix(fmt.Sprintf("%s: decompressing %s ",
				cmdName, arg))
		} else {
			log.SetPrefix(fmt.Sprintf("%s: compressing %s ",
				cmdName, arg))
		}
		if err := processLZMA(opts, arg); err != nil {
			if opts.verbose >= 2 {
				log.Printf("exit after error %s", err)
			}
			os.Exit(3)
		}
	}
}
