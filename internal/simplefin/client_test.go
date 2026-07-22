package simplefin

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClaim(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("claim must POST, got %s", r.Method)
		}
		w.Write([]byte("https://user:pass@bridge.example.com/simplefin"))
	}))
	defer srv.Close()

	setupToken := base64.StdEncoding.EncodeToString([]byte(srv.URL))
	got, err := Claim(setupToken)
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://user:pass@bridge.example.com/simplefin" {
		t.Fatalf("got %q", got)
	}
}

func TestAccounts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != "user" || p != "pass" {
			t.Errorf("basic auth not forwarded (ok=%v u=%q)", ok, u)
		}
		if r.URL.Path != "/accounts" {
			t.Errorf("path %q", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("start-date") == "" || q.Get("pending") != "1" {
			t.Errorf("missing params: %v", q)
		}
		w.Write([]byte(`{
		  "errors": ["Connection to Fake Bank may need attention"],
		  "accounts": [{
		    "id": "ACT-123", "name": "Checking", "currency": "USD",
		    "balance": "1204.55", "balance-date": 1784505600,
		    "org": {"name": "Ally Bank", "domain": "ally.com"},
		    "transactions": [
		      {"id": "TXN-1", "posted": 1784419200, "amount": "-14.62", "description": "CHIPOTLE 2291", "pending": false}
		    ]
		  }]
		}`))
	}))
	defer srv.Close()

	c := New("https://user:pass@" + srv.Listener.Addr().String())
	c.scheme = "http" // test-only override; real bridge is always https
	set, err := c.Accounts(context.Background(), time.Unix(1750000000, 0), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Errors) != 1 || len(set.Accounts) != 1 {
		t.Fatalf("errors=%v accounts=%d", set.Errors, len(set.Accounts))
	}
	a := set.Accounts[0]
	if a.ID != "ACT-123" || a.Org.Name != "Ally Bank" || a.Balance != "1204.55" {
		t.Fatalf("account parsed wrong: %+v", a)
	}
	if len(a.Transactions) != 1 || a.Transactions[0].Amount != "-14.62" {
		t.Fatalf("txns parsed wrong: %+v", a.Transactions)
	}
	if a.Transactions[0].PostedTime().Format("2006-01-02") != "2026-07-19" {
		t.Fatalf("posted time wrong: %v", a.Transactions[0].PostedTime())
	}
}
