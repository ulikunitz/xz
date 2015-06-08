package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

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

func main() {
	// initialization
	cmdName := filepath.Base(os.Args[0])
	pflag.CommandLine = pflag.NewFlagSet(cmdName, pflag.ExitOnError)
	pflag.SetInterspersed(true)
	pflag.Usage = func() { usage(os.Stderr); os.Exit(1) }
	log.SetPrefix(fmt.Sprintf("%s: ", cmdName))
	log.SetFlags(0)

	var (
		help   = pflag.BoolP("help", "h", false, "")
		preset = defaultPreset
	)

	preset.filter()
	log.Printf("filtered args %v", os.Args)
	pflag.Parse()

	if *help {
		usage(os.Stdout)
		os.Exit(0)
	}

	log.Printf("preset %d", preset)
}
