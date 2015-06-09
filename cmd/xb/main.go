package main

//go:generate goversion -o version.go

import (
	"fmt"
	"log"
	"os"
)

const usage = `xb <command> 

xb is a supporting building tool from the xz project for Go.

  xb help         -- prints this message 
  xb version-file -- generates go file with version information
  xb cat          -- generates go file that includes the given text files
  xb version      -- prints version information for xb

Report bugs using <https://github.com/uli-go/xz/issues>.
`

func updateArgs(cmd string) {
	args := make([]string, 1, len(os.Args)-1)
	args[0] = "xb " + cmd
	os.Args = append(args, os.Args[2:]...)
}

func main() {
	log.SetPrefix("xb")
	log.SetFlags(0)

	if len(os.Args) < 2 {
		log.Fatal("to show help, use xb help")
	}

	switch os.Args[1] {
	case "help", "-h":
		fmt.Print(usage)
		os.Exit(0)
	case "version":
		fmt.Printf("xb %s\n", version)
		os.Exit(0)
	case "cat":
		updateArgs("cat")
		fmt.Printf("os.Args: %v\n", os.Args)
		os.Exit(0)
	case "version-file":
		updateArgs("version-file")
		fmt.Printf("os.Args: %v\n", os.Args)
		os.Exit(0)
	default:
		log.Fatalf("command %q not supported", os.Args[1])
	}
}
