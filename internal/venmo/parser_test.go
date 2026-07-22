package venmo

import (
	"os"
	"testing"
)

func TestParseGoldenFile(t *testing.T) {
	f, err := os.Open("testdata/venmo-2026.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	txns, err := Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(txns) != 3 {
		t.Fatalf("want 3 txns, got %d", len(txns))
	}

	out := txns[0]
	if out.ExternalID != "4111111111111111111" || out.Amount != "-32.00" ||
		out.Payee != "Luigi Mario" || out.Description != "Pizza night" ||
		out.Posted.Format("2006-01-02") != "2026-07-15" ||
		out.AccountID != "venmo" || out.Source != "venmo_csv" {
		t.Errorf("outgoing parsed wrong: %+v", out)
	}
	if fs := FundingSource(out.Raw); fs != "Ally Bank Personal Checking x1234" {
		t.Errorf("funding source: %q", fs)
	}

	in := txns[2]
	if in.Amount != "25.00" || in.Payee != "Dave Webb" {
		t.Errorf("incoming parsed wrong: %+v", in)
	}
}

func TestParseRejectsMissingColumns(t *testing.T) {
	f, _ := os.CreateTemp(t.TempDir(), "bad*.csv")
	f.WriteString("Nope,Nada\n1,2\n")
	f.Seek(0, 0)
	if _, err := Parse(f); err == nil {
		t.Fatal("expected error for unrecognizable CSV")
	}
}
