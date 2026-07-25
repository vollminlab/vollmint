package venmo

import (
	"os"
	"strings"
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
	if out.Pending {
		t.Errorf("want Pending=false (Status=Complete), got true")
	}
	if fs := FundingSource(out.Raw); fs != "Example Bank Personal Checking x1234" {
		t.Errorf("funding source: %q", fs)
	}

	in := txns[2]
	if in.Amount != "25.00" || in.Payee != "Pat Peer" {
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

func TestParsePendingStatus(t *testing.T) {
	csv := `,ID,Datetime,Type,Status,Note,From,To,Amount (total),Amount (tip),Amount (tax),Amount (fee),Tax Rate,Tax Exempt,Funding Source,Destination,Beginning Balance,Ending Balance,Statement Period Venmo Fees,Terminal Location,Year to Date Venmo Fees,Disclaimer
,5555555555555555555,2026-07-14T10:00:00,Payment,Pending,Test pending,Sam Sample,Test User,- $10.00,,,,,,Venmo balance,,,,,Venmo,,`

	txns, err := Parse(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(txns) != 1 {
		t.Fatalf("want 1 txn, got %d", len(txns))
	}
	if !txns[0].Pending {
		t.Errorf("want Pending=true (Status=Pending), got false")
	}
}

func TestParseMalformedAmount(t *testing.T) {
	csv := `,ID,Datetime,Type,Status,Note,From,To,Amount (total),Amount (tip),Amount (tax),Amount (fee),Tax Rate,Tax Exempt,Funding Source,Destination,Beginning Balance,Ending Balance,Statement Period Venmo Fees,Terminal Location,Year to Date Venmo Fees,Disclaimer
,9999999999999999999,2026-07-14T10:00:00,Payment,Complete,Bad amount,Sam Sample,Test,$1.2.3,,,,,,Venmo balance,,,,,Venmo,,`

	_, err := Parse(strings.NewReader(csv))
	if err == nil {
		t.Fatal("expected error for malformed amount")
	}
	if !strings.Contains(err.Error(), "9999999999999999999") {
		t.Errorf("error should mention row ID, got: %v", err)
	}
}

func TestParseMalformedDatetime(t *testing.T) {
	csv := `,ID,Datetime,Type,Status,Note,From,To,Amount (total),Amount (tip),Amount (tax),Amount (fee),Tax Rate,Tax Exempt,Funding Source,Destination,Beginning Balance,Ending Balance,Statement Period Venmo Fees,Terminal Location,Year to Date Venmo Fees,Disclaimer
,8888888888888888888,07/15/2026,Payment,Complete,Bad date,Sam Sample,Test,- $5.00,,,,,,Venmo balance,,,,,Venmo,,`

	_, err := Parse(strings.NewReader(csv))
	if err == nil {
		t.Fatal("expected error for malformed datetime")
	}
	if !strings.Contains(err.Error(), "8888888888888888888") {
		t.Errorf("error should mention row ID, got: %v", err)
	}
}
