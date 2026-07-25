package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestGetTransactions(t *testing.T) {
	s := testStore(t)
	seedTxn(t, s, "ally-s", "scott", "s1", "2026-07-05", "-10.00", "Coffee")
	seedTxn(t, s, "ally-n", "nikki", "n1", "2026-07-06", "-20.00", "Books")
	srv := New(s)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/transactions?view=household&month=2026-07", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Transactions []struct {
			ID     int64  `json:"id"`
			Amount string `json:"amount"`
		} `json:"transactions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Transactions) != 2 {
		t.Fatalf("got %d transactions, want 2", len(body.Transactions))
	}
}

func TestGetTransactionsRejectsBadMonth(t *testing.T) {
	srv := New(testStore(t))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/transactions?view=household&month=julyish", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad month status = %d, want 400", rec.Code)
	}
}

func TestGetTransactionsRejectsBadView(t *testing.T) {
	srv := New(testStore(t))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/transactions?view=admin", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad view status = %d, want 400", rec.Code)
	}
}

func TestGetTransactionsRejectsBadCategory(t *testing.T) {
	srv := New(testStore(t))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/transactions?category=abc", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad category status = %d, want 400", rec.Code)
	}
}

func TestGetTransactionsSemanticallyInvalidMonth(t *testing.T) {
	srv := New(testStore(t))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/transactions?month=2026-13", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("semantically invalid month status = %d, want 400", rec.Code)
	}
}

func TestPatchTransactionSetsCategory(t *testing.T) {
	s := testStore(t)
	id := seedTxn(t, s, "joint1", "joint", "j1", "2026-07-10", "-40.00", "Dinner")
	var diningID int
	_ = s.Pool.QueryRow(nil0(), `SELECT id FROM categories WHERE name='Dining'`).Scan(&diningID)
	srv := New(s)

	body := strings.NewReader(`{"category_id": ` + itoa(diningID) + `}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/transactions/"+itoa64(id), body)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPatchTransactionNotFound(t *testing.T) {
	srv := New(testStore(t))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/transactions/999999", strings.NewReader(`{"category_id":1}`))
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", rec.Code)
	}
}

func TestPatchTransactionBadID(t *testing.T) {
	srv := New(testStore(t))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/transactions/abc", strings.NewReader(`{}`))
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}

func itoa(i int) string    { return strconv.Itoa(i) }
func itoa64(i int64) string { return strconv.FormatInt(i, 10) }
func nil0() context.Context { return context.Background() }
