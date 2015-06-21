package main

//go:generate xb cat -o licenses.go xzLicense:github.com/uli-go/xz/LICENSE goLicense:~/go/LICENSE
//go:generate xb version-file -o version.go

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/uli-go/xz/gflag"
	"github.com/uli-go/xz/term"
	"github.com/uli-go/xz/xlog"
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

Go Programming Language
=======================

The lzmago program contains the packages gflag and xlog that are
extensions of packages from the Go standard library. The packages may
contain code from those packages.

{{.go}}
`
	out = strings.TrimLeft(out, " \n")
	tmpl, err := template.New("licenses").Parse(out)
	if err != nil {
		xlog.Panicf("error %s parsing licenses template", err)
	}
	lmap := map[string]string{
		"xz": strings.TrimSpace(xzLicense),
		"go": strings.TrimSpace(goLicense),
	}
	if err = tmpl.Execute(w, lmap); err != nil {
		xlog.Fatalf("error %s writing licenses template", err)
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
		xlog.Panicf("options are already initialized")
	}
	gflag.BoolVarP(&o.help, "help", "h", false, "")
	gflag.BoolVarP(&o.stdout, "stdout", "c", false, "")
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
	xlog.SetPrefix(fmt.Sprintf("%s: ", cmdName))
	xlog.SetFlags(0)

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
		xlog.Printf("version %s\n", version)
		os.Exit(0)
	}

	flags := xlog.Flags()
	switch {
	case opts.verbose <= 0:
		flags |= xlog.Lnoprint | xlog.Lnodebug
	case opts.verbose == 1:
		flags |= xlog.Lnodebug
	}
	switch {
	case opts.quiet >= 2:
		flags |= xlog.Lnoprint | xlog.Lnowarn | xlog.Lnodebug
		flags |= xlog.Lnopanic | xlog.Lnofatal
	case opts.quiet == 1:
		flags |= xlog.Lnoprint | xlog.Lnowarn | xlog.Lnodebug
	}
	xlog.SetFlags(flags)

	var args []string
	if gflag.NArg() == 0 {
		opts.stdout = true
		args = []string{"-"}
	} else {
		args = gflag.Args()
	}

	if opts.stdout && !opts.decompress && !opts.force &&
		term.IsTerminal(os.Stdout.Fd()) {
		xlog.Fatal(`Compressed data will not be written to a terminal
Use -f to force compression. For help type lzmago -h.`)
	}

	for _, arg := range args {
		processFile(arg, &opts)
	}
}
