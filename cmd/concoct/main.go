package main

import (
	"fmt"
	"io"
	"os"
)

const version = "0.0.0-dev (CON-043 reset skeleton)"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stdout)
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "concoct: help takes no arguments")
			return 2
		}
		usage(stdout)
		return 0
	case "version", "-v", "--version":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "concoct: version takes no arguments")
			return 2
		}
		fmt.Fprintln(stdout, version)
		return 0
	default:
		fmt.Fprintf(stderr, "concoct: %q is unavailable in the CON-043 reset skeleton; only help and version are implemented\n", args[0])
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "Concoct CON-043 reset skeleton")
	fmt.Fprintln(w, "Usage: concoct [help|version]")
	fmt.Fprintln(w, "Workflow commands are not implemented yet.")
}
