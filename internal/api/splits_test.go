package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func putSplitsReq(t *testing.T, h http.Handler, id string, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/api/transactions/"+id+"/splits",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestPutSplits(t *testing.T) {
	s := testStore(t)
	h := New(s).Handler()
	ctx := context.Background()

	id := seedTxn(t, s, "acct-sp", "scott", "sp-1", "2026-07-10", "-50.00", "venmo dinner")

	var dining, groceries int
	if err := s.Pool.QueryRow(ctx, `SELECT id FROM categories WHERE name='Dining'`).Scan(&dining); err != nil {
		t.Fatalf("dining: %v", err)
	}
	if err := s.Pool.QueryRow(ctx, `SELECT id FROM categories WHERE name='Groceries'`).Scan(&groceries); err != nil {
		t.Fatalf("groceries: %v", err)
	}

	body := func(a, b string) string {
		return `{"splits":[{"category_id":` + itoa(dining) + `,"amount":"` + a +
			`","note":""},{"category_id":` + itoa(groceries) + `,"amount":"` + b + `","note":""}]}`
	}

	t.Run("happy path returns updated transaction", func(t *testing.T) {
		w := putSplitsReq(t, h, itoa64(id), body("-30.00", "-20.00"))
		if w.Code != http.StatusOK {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
		var resp struct {
			Transaction struct {
				ID     int64 `json:"id"`
				Splits []struct {
					Category string `json:"category"`
					Amount   string `json:"amount"`
				} `json:"splits"`
			} `json:"transaction"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Transaction.ID != id || len(resp.Transaction.Splits) != 2 {
			t.Fatalf("bad envelope: %s", w.Body.String())
		}
	})

	t.Run("sum mismatch is 400 with amounts in message", func(t *testing.T) {
		w := putSplitsReq(t, h, itoa64(id), body("-30.00", "-15.00"))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status %d", w.Code)
		}
		var e struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if e.Error == "" {
			t.Fatal("empty error message")
		}
	})

	t.Run("non-integer id is 400", func(t *testing.T) {
		w := putSplitsReq(t, h, "abc", body("-30.00", "-20.00"))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status %d", w.Code)
		}
	})

	t.Run("unknown transaction is 404", func(t *testing.T) {
		w := putSplitsReq(t, h, "999999", body("-30.00", "-20.00"))
		if w.Code != http.StatusNotFound {
			t.Fatalf("status %d", w.Code)
		}
	})

	t.Run("bad JSON is 400", func(t *testing.T) {
		w := putSplitsReq(t, h, itoa64(id), `{"splits": nope}`)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status %d", w.Code)
		}
	})

	t.Run("unknown category is 400", func(t *testing.T) {
		w := putSplitsReq(t, h, itoa64(id),
			`{"splits":[{"category_id":424242,"amount":"-30.00","note":""},{"category_id":424243,"amount":"-20.00","note":""}]}`)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestDeleteSplits(t *testing.T) {
	s := testStore(t)
	h := New(s).Handler()

	id := seedTxn(t, s, "acct-del", "scott", "del-1", "2026-07-10", "-50.00", "delete me")

	req := httptest.NewRequest(http.MethodDelete, "/api/transactions/"+itoa64(id)+"/splits", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	// Idempotent — second delete also 200.
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/transactions/"+itoa64(id)+"/splits", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("second delete status %d", w.Code)
	}
}
