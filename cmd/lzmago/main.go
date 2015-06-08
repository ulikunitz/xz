package main

//go:generate gocat -o licenses.go xzLicense:github.com/uli-go/xz/LICENSE pflagLicense:github.com/ogier/pflag/LICENSE

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/ogier/pflag"
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

type Preset int

const defaultPreset Preset = 6

func (p *Preset) filterArg(arg string) string {
	if len(arg) < 2 || arg[0] != '-' || arg[1] == '-' {
		return arg
	}
	buf := new(bytes.Buffer)
	buf.Grow(len(arg))
	for _, c := range arg {
		if '0' <= c && c <= '9' {
			*p = Preset(c - '0')
			continue
		}
		buf.WriteRune(c)
	}
	return buf.String()
}

func (p *Preset) filter() {
	args := make([]string, 1, len(os.Args))
	args[0] = os.Args[0]
	for i, arg := range os.Args[1:] {
		if arg == "--" {
			args = append(args, os.Args[1+i:]...)
			break

		}
		arg = p.filterArg(arg)
		if arg != "-" {
			args = append(args, arg)
		}
	}
	os.Args = args
}

func usage(w io.Writer) {
	fmt.Fprint(w, usageStr)
}

func licenses(w io.Writer) {
	out := `
github.com/uli-go/xz -- xz for Go 
=================================

{{.xz}}

pflag -- Posix flag package
===========================

{{.pflag}}
`
	out = strings.TrimLeft(out, " \n")
	tmpl, err := template.New("licenses").Parse(out)
	if err != nil {
		log.Panicf("error %s parsing licenses template", err)
	}
	lmap := map[string]string{
		"xz":    strings.TrimSpace(xzLicense),
		"pflag": strings.TrimSpace(pflagLicense),
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
	pflag.CommandLine = pflag.NewFlagSet(cmdName, pflag.ExitOnError)
	pflag.SetInterspersed(true)
	pflag.Usage = func() { usage(os.Stderr); os.Exit(1) }
	var (
		help       = pflag.BoolP("help", "h", false, "")
		stdout     = pflag.BoolP("stdout", "c", false, "")
		decompress = pflag.BoolP("decompress", "d", false, "")
		force      = pflag.BoolP("force", "f", false, "")
		keep       = pflag.BoolP("keep", "k", false, "")
		license    = pflag.BoolP("license", "L", false, "")
		preset     = defaultPreset
	)

	// process arguments
	preset.filter()
	log.Printf("filtered args %v", os.Args)
	pflag.Parse()

	if *help {
		usage(os.Stdout)
		os.Exit(0)
	}
	if *license {
		licenses(os.Stdout)
		os.Exit(0)
	}
	if pflag.NArg() == 0 {
		log.Fatal("for help, type lzmago -h")
	}

	log.Printf("decompress %t", *decompress)
	log.Printf("force %t", *force)
	log.Printf("keep %t", *keep)
	log.Printf("preset %d", preset)
	log.Printf("stdout %t", *stdout)
}
