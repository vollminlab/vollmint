package main

import (
	"context"
	"fmt"
	"os"

	"github.com/vollminlab/vollmint/internal/ingest"
	"github.com/vollminlab/vollmint/internal/migrate"
	"github.com/vollminlab/vollmint/internal/simplefin"
	"github.com/vollminlab/vollmint/internal/store"
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

func runSync(args []string) error {
	dbURL := os.Getenv("DATABASE_URL")
	accessURL := os.Getenv("SIMPLEFIN_ACCESS_URL")
	if dbURL == "" || accessURL == "" {
		return fmt.Errorf("DATABASE_URL and SIMPLEFIN_ACCESS_URL are required")
	}
	ctx := context.Background()
	if err := migrate.Up(dbURL); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	s, err := store.New(ctx, dbURL)
	if err != nil {
		return err
	}
	defer s.Close()
	res, err := ingest.Sync(ctx, s, simplefin.New(accessURL), "scott")
	if err != nil {
		return err
	}
	fmt.Printf("sync ok: upserted=%d categorized=%d paired=%d swept=%d\n",
		res.Upserted, res.Categorized, res.Paired, res.Swept)
	return nil
}

func runImportVenmo(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: vollmint import-venmo <statement.csv>")
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	ctx := context.Background()
	if err := migrate.Up(dbURL); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	s, err := store.New(ctx, dbURL)
	if err != nil {
		return err
	}
	defer s.Close()
	f, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer f.Close()
	res, err := ingest.ImportVenmo(ctx, s, f)
	if err != nil {
		return err
	}
	fmt.Printf("import ok: upserted=%d categorized=%d paired=%d\n",
		res.Upserted, res.Categorized, res.Paired)
	fmt.Fprintln(os.Stderr, "Reminder: delete the CSV export when done — it is not retained by vollmint.")
	return nil
}
