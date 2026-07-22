// Package venmo parses Venmo's desktop-web CSV statement export.
// The format drifts over time, so columns are located by header NAME, never
// by position. Rows without an ID (title/summary lines) are skipped.
package venmo

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/vollminlab/vollmint/internal/store"
)

var amountClean = regexp.MustCompile(`[^0-9.\-]`)

func Parse(r io.Reader) ([]store.Txn, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1 // Venmo pads rows inconsistently

	var header map[string]int
	var txns []store.Txn
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("csv read: %w", err)
		}
		if header == nil {
			if idx := indexHeader(rec); idx != nil {
				header = idx
			}
			continue // still hunting for the header row (title lines precede it)
		}
		get := func(col string) string {
			i, ok := header[col]
			if !ok || i >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[i])
		}
		id := get("ID")
		if id == "" {
			continue // summary/blank row
		}
		posted, err := time.Parse("2006-01-02T15:04:05", get("Datetime"))
		if err != nil {
			return nil, fmt.Errorf("row %s: bad datetime %q", id, get("Datetime"))
		}
		amount, err := parseAmount(get("Amount (total)"))
		if err != nil {
			return nil, fmt.Errorf("row %s: %w", id, err)
		}
		payee := get("To")
		if !strings.HasPrefix(amount, "-") {
			payee = get("From")
		}
		raw, _ := json.Marshal(map[string]string{
			"type": get("Type"), "status": get("Status"),
			"from": get("From"), "to": get("To"),
			"funding_source": get("Funding Source"), "destination": get("Destination"),
		})
		txns = append(txns, store.Txn{
			Source: "venmo_csv", ExternalID: id, AccountID: "venmo",
			Posted: posted.UTC(), Amount: amount,
			Description: get("Note"), Payee: payee, Raw: raw,
			Pending: strings.EqualFold(get("Status"), "Pending"),
		})
	}
	if header == nil {
		return nil, fmt.Errorf("no Venmo header row found (need ID, Datetime, Amount (total) columns)")
	}
	return txns, nil
}

// indexHeader returns a name→index map if rec looks like the Venmo header row.
func indexHeader(rec []string) map[string]int {
	idx := map[string]int{}
	for i, name := range rec {
		if n := strings.TrimSpace(name); n != "" {
			idx[n] = i
		}
	}
	for _, required := range []string{"ID", "Datetime", "Amount (total)"} {
		if _, ok := idx[required]; !ok {
			return nil
		}
	}
	return idx
}

// parseAmount turns "- $1,234.50" / "+ $25.00" into "-1234.50" / "25.00".
func parseAmount(s string) (string, error) {
	if s == "" {
		return "", fmt.Errorf("empty amount")
	}
	neg := strings.Contains(s, "-")
	cleaned := amountClean.ReplaceAllString(s, "")
	cleaned = strings.TrimPrefix(cleaned, "-")
	if cleaned == "" || strings.Count(cleaned, ".") > 1 {
		return "", fmt.Errorf("unparseable amount %q", s)
	}
	if neg {
		cleaned = "-" + cleaned
	}
	return cleaned, nil
}

// FundingSource extracts the funding source from a parsed row's raw json.
// Diagnostic surface for the UI ("funded by Ally" vs "Venmo balance") —
// the matcher itself pairs purely on amount + date.
func FundingSource(raw []byte) string {
	var m map[string]string
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	return m["funding_source"]
}
