package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vollminlab/vollmint/internal/store"
)

// seedAccount inserts a bare active account (no balance, no snapshots).
func seedAccount(t *testing.T, s *store.Store, id, owner string) {
	t.Helper()
	if _, err := s.Pool.Exec(context.Background(), `
		INSERT INTO accounts (id, name, org, currency, owner)
		VALUES ($1, $1, 't', 'USD', $2)`, id, owner); err != nil {
		t.Fatal(err)
	}
}

// seedSnap upserts a snapshot daysAgo days before CURRENT_DATE.
func seedSnap(t *testing.T, s *store.Store, id string, daysAgo int, balance string) {
	t.Helper()
	if _, err := s.Pool.Exec(context.Background(), `
		INSERT INTO account_balance_snapshots (account_id, snapshot_date, balance)
		VALUES ($1, CURRENT_DATE - $2::int, $3::numeric)
		ON CONFLICT (account_id, snapshot_date) DO UPDATE SET balance = EXCLUDED.balance`,
		id, daysAgo, balance); err != nil {
		t.Fatal(err)
	}
}

type networthBody struct {
	Series []struct {
		Date     string            `json:"date"`
		Total    string            `json:"total"`
		Accounts map[string]string `json:"accounts"`
	} `json:"series"`
	Accounts []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Owner       string `json:"owner"`
		IsManual    bool   `json:"is_manual"`
		Balance     string `json:"balance"`
		BalanceDate string `json:"balance_date"`
	} `json:"accounts"`
}

func getNetworth(t *testing.T, srv *Server, query string) (int, networthBody) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/networth"+query, nil)
	srv.Handler().ServeHTTP(rec, req)
	var body networthBody
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v body=%s", err, rec.Body.String())
		}
	}
	return rec.Code, body
}

func TestNetWorthCarryForward(t *testing.T) {
	s := testStore(t)
	seedAccount(t, s, "a1", "scott")
	seedSnap(t, s, "a1", 4, "100.00")
	seedSnap(t, s, "a1", 1, "200.00")
	srv := New(s)

	code, body := getNetworth(t, srv, "?view=scott&range=1m")
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	// Axis starts at a1's first snapshot (4 days ago), ends today: 5 points.
	if len(body.Series) != 5 {
		t.Fatalf("series len = %d, want 5: %+v", len(body.Series), body.Series)
	}
	if body.Series[0].Total != "100.00" {
		t.Errorf("day-4 total = %s, want 100.00", body.Series[0].Total)
	}
	// Day-3 has no snapshot — carry-forward from day-4.
	if body.Series[1].Total != "100.00" {
		t.Errorf("day-3 (carry-forward) total = %s, want 100.00", body.Series[1].Total)
	}
	if body.Series[3].Total != "200.00" {
		t.Errorf("day-1 total = %s, want 200.00", body.Series[3].Total)
	}
	// Today has no snapshot — carries forward day-1's value.
	if body.Series[4].Total != "200.00" {
		t.Errorf("today (carry-forward) total = %s, want 200.00", body.Series[4].Total)
	}
	if got := body.Series[4].Accounts["a1"]; got != "200.00" {
		t.Errorf("today a1 = %s, want 200.00", got)
	}
}

func TestNetWorthViewFilter(t *testing.T) {
	s := testStore(t)
	seedAccount(t, s, "a1", "scott")
	seedAccount(t, s, "a2", "nikki")
	seedSnap(t, s, "a1", 0, "100.00")
	seedSnap(t, s, "a2", 0, "50.00")
	srv := New(s)

	code, body := getNetworth(t, srv, "?view=scott&range=1m")
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	last := body.Series[len(body.Series)-1]
	if last.Total != "100.00" {
		t.Errorf("scott total = %s, want 100.00", last.Total)
	}
	if _, ok := last.Accounts["a2"]; ok {
		t.Error("nikki account a2 leaked into scott series")
	}
	ids := map[string]bool{}
	for _, a := range body.Accounts {
		ids[a.ID] = true
	}
	if !ids["a1"] || ids["a2"] {
		t.Errorf("scott account list wrong: %v", ids)
	}

	code, body = getNetworth(t, srv, "?view=household&range=1m")
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	last = body.Series[len(body.Series)-1]
	if last.Total != "150.00" {
		t.Errorf("household total = %s, want 150.00", last.Total)
	}
}

func TestNetWorthExcludesInactive(t *testing.T) {
	s := testStore(t)
	seedAccount(t, s, "a1", "scott")
	seedAccount(t, s, "dead", "scott")
	seedSnap(t, s, "a1", 0, "100.00")
	seedSnap(t, s, "dead", 0, "999.00")
	if _, err := s.Pool.Exec(context.Background(),
		`UPDATE accounts SET active = false WHERE id = 'dead'`); err != nil {
		t.Fatal(err)
	}
	srv := New(s)

	_, body := getNetworth(t, srv, "?view=scott&range=1m")
	last := body.Series[len(body.Series)-1]
	if last.Total != "100.00" {
		t.Errorf("total = %s, want 100.00 (inactive excluded)", last.Total)
	}
	for _, a := range body.Accounts {
		if a.ID == "dead" {
			t.Error("inactive account listed")
		}
	}
}

func TestNetWorthEmptyAndValidation(t *testing.T) {
	s := testStore(t)
	seedAccount(t, s, "a1", "scott")
	srv := New(s)

	// No snapshots anywhere: empty (non-null) series, populated accounts.
	code, body := getNetworth(t, srv, "?view=scott")
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if body.Series == nil || len(body.Series) != 0 {
		t.Errorf("series = %v, want empty non-null", body.Series)
	}
	ids := map[string]bool{}
	for _, a := range body.Accounts {
		ids[a.ID] = true
	}
	if !ids["a1"] {
		t.Error("account list missing a1")
	}

	if code, _ := getNetworth(t, srv, "?view=bogus"); code != http.StatusBadRequest {
		t.Errorf("bad view status = %d, want 400", code)
	}
	if code, _ := getNetworth(t, srv, "?view=scott&range=2w"); code != http.StatusBadRequest {
		t.Errorf("bad range status = %d, want 400", code)
	}
}

func doJSON(t *testing.T, srv *Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

func TestCreateManualAccountEndpoint(t *testing.T) {
	s := testStore(t)
	srv := New(s)

	rec := doJSON(t, srv, http.MethodPost, "/api/accounts/manual",
		`{"name":"401k","owner":"scott","balance":"412000.00"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID != "manual-401k" {
		t.Errorf("id = %s, want manual-401k", created.ID)
	}

	// Validation matrix.
	for name, tc := range map[string]struct {
		body string
		want int
	}{
		"duplicate":    {`{"name":"401k","owner":"scott","balance":"1.00"}`, http.StatusConflict},
		"bad owner":    {`{"name":"X","owner":"household","balance":"1.00"}`, http.StatusBadRequest},
		"bad balance":  {`{"name":"Y","owner":"scott","balance":"1.2.3"}`, http.StatusBadRequest},
		"empty name":   {`{"name":"  ","owner":"scott","balance":"1.00"}`, http.StatusBadRequest},
		"symbols name": {`{"name":"!!!","owner":"scott","balance":"1.00"}`, http.StatusBadRequest},
		"bad json":     {`{`, http.StatusBadRequest},
	} {
		rec := doJSON(t, srv, http.MethodPost, "/api/accounts/manual", tc.body)
		if rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d (body=%s)", name, rec.Code, tc.want, rec.Body.String())
		}
	}
}

func TestUpdateManualBalanceEndpoint(t *testing.T) {
	s := testStore(t)
	if _, err := s.CreateManualAccount(context.Background(), "401k", "scott", "412000.00"); err != nil {
		t.Fatal(err)
	}
	seedAccount(t, s, "synced-1", "scott")
	srv := New(s)

	rec := doJSON(t, srv, http.MethodPut, "/api/accounts/manual-401k/balance", `{"balance":"415250.00"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var balance string
	if err := s.Pool.QueryRow(context.Background(),
		`SELECT balance::text FROM accounts WHERE id = 'manual-401k'`).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if balance != "415250.00" {
		t.Errorf("balance = %s, want 415250.00", balance)
	}

	if rec := doJSON(t, srv, http.MethodPut, "/api/accounts/synced-1/balance", `{"balance":"1.00"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("synced: status = %d, want 400", rec.Code)
	}
	if rec := doJSON(t, srv, http.MethodPut, "/api/accounts/nope/balance", `{"balance":"1.00"}`); rec.Code != http.StatusNotFound {
		t.Errorf("unknown: status = %d, want 404", rec.Code)
	}
	if rec := doJSON(t, srv, http.MethodPut, "/api/accounts/manual-401k/balance", `{"balance":"xx"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("bad balance: status = %d, want 400", rec.Code)
	}
}
