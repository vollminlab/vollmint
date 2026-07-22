package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "claim":
		err = runClaim(os.Args[2:])
	case "sync":
		err = runSync(os.Args[2:])
	case "import-venmo":
		err = runImportVenmo(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: vollmint <claim|sync|import-venmo> [args]")
	os.Exit(2)
}

// Implemented in later tasks; stubs keep the build green.
func runClaim(args []string) error       { return fmt.Errorf("not implemented") }
func runSync(args []string) error        { return fmt.Errorf("not implemented") }
func runImportVenmo(args []string) error { return fmt.Errorf("not implemented") }
