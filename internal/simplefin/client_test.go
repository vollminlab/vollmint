package simplefin

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestClaimRejectsBadToken(t *testing.T) {
	_, err := Claim("not-base64!!!")
	if err == nil {
		t.Fatal("expected error for bad token, got nil")
	}
	if !strings.Contains(err.Error(), "base64") {
		t.Fatalf("error should mention base64, got: %v", err)
	}
}

func TestClaimNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("denied"))
	}))
	defer srv.Close()

	setupToken := base64.StdEncoding.EncodeToString([]byte(srv.URL))
	_, err := Claim(setupToken)
	if err == nil {
		t.Fatal("expected error for non-200, got nil")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Fatalf("error should mention 403, got: %v", err)
	}
}

func TestNewDegradedClient(t *testing.T) {
	tests := []struct {
		name       string
		accessURL  string
	}{
		{"malformed URL", "://not-a-url"},
		{"valid URL but no userinfo", "https://bridge.example.com/simplefin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New(tt.accessURL)
			_, err := c.Accounts(context.Background(), time.Unix(0, 0), false)
			if err == nil {
				t.Fatal("expected error for degraded client, got nil")
			}
			if !strings.Contains(err.Error(), "invalid SimpleFIN access URL") {
				t.Fatalf("error should mention invalid SimpleFIN access URL, got: %v", err)
			}
		})
	}
}

func TestAccountsNon200DoesNotLeakCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("bad auth"))
	}))
	defer srv.Close()

	c := New("https://secretuser:secretpass@" + srv.Listener.Addr().String())
	c.scheme = "http" // test-only override; real bridge is always https
	_, err := c.Accounts(context.Background(), time.Unix(0, 0), false)
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("error should mention 401, got: %v", err)
	}
	if strings.Contains(err.Error(), "secretpass") {
		t.Fatalf("error leaked secretpass: %v", err)
	}
	if strings.Contains(err.Error(), "secretuser") {
		t.Fatalf("error leaked secretuser: %v", err)
	}
}
