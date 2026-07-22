package main

import (
	"fmt"
	"os"

	"github.com/vollminlab/vollmint/internal/simplefin"
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

func runClaim(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: vollmint claim <setup-token>")
	}
	accessURL, err := simplefin.Claim(args[0])
	if err != nil {
		return err
	}
	fmt.Println(accessURL)
	fmt.Fprintln(os.Stderr, "\nSave this Access URL to 1Password now (item \"SimpleFIN Access URL\", field: token).")
	fmt.Fprintln(os.Stderr, "Do NOT write it to any file. The setup token above is now spent.")
	return nil
}

// Implemented in later tasks; stubs keep the build green.
func runSync(args []string) error        { return fmt.Errorf("not implemented") }
func runImportVenmo(args []string) error { return fmt.Errorf("not implemented") }
