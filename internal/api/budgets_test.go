package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPutAndGetBudgets(t *testing.T) {
	s := testStore(t)
	srv := New(s)

	var groceriesID int
	_ = s.Pool.QueryRow(context.Background(), `SELECT id FROM categories WHERE name='Groceries'`).Scan(&groceriesID)

	// PUT budgets
	rec := httptest.NewRecorder()
	body := `{"budgets":[{"category_id":` + itoa(groceriesID) + `,"amount":"120.00"}]}`
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/budgets?month=2026-07", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("put status=%d body=%s", rec.Code, rec.Body.String())
	}

	// GET budgets
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/budgets?month=2026-07", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get status=%d", rec.Code)
	}
	var getResp struct {
		Budgets []struct {
			Amount string `json:"amount"`
		} `json:"budgets"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &getResp)
	if len(getResp.Budgets) != 1 || getResp.Budgets[0].Amount != "120.00" {
		t.Fatalf("unexpected budgets in response: %+v", getResp.Budgets)
	}
}

func TestPutBudgetsBadAmount(t *testing.T) {
	s := testStore(t)
	srv := New(s)

	var groceriesID int
	_ = s.Pool.QueryRow(context.Background(), `SELECT id FROM categories WHERE name='Groceries'`).Scan(&groceriesID)

	rec := httptest.NewRecorder()
	body := `{"budgets":[{"category_id":` + itoa(groceriesID) + `,"amount":"12.3.4"}]}`
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/budgets?month=2026-07", strings.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}

func TestGetBudgetsMissingMonth(t *testing.T) {
	srv := New(testStore(t))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/budgets", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}

func TestGetBudgetsBadMonth(t *testing.T) {
	srv := New(testStore(t))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/budgets?month=2026-13", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}

func TestPutBudgetsBadMonth(t *testing.T) {
	srv := New(testStore(t))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/budgets?month=2026-13", strings.NewReader(`{"budgets":[]}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}

func TestPutBudgetsBadJSON(t *testing.T) {
	srv := New(testStore(t))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/budgets?month=2026-07", strings.NewReader(`{not json`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}

func TestPutBudgetsBadCategoryFK(t *testing.T) {
	srv := New(testStore(t))
	rec := httptest.NewRecorder()
	body := `{"budgets":[{"category_id":999999,"amount":"120.00"}]}`
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/budgets?month=2026-07", strings.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}

func TestPutBudgetsDuplicateCategory(t *testing.T) {
	s := testStore(t)
	srv := New(s)

	var groceriesID int
	_ = s.Pool.QueryRow(context.Background(), `SELECT id FROM categories WHERE name='Groceries'`).Scan(&groceriesID)

	rec := httptest.NewRecorder()
	body := `{"budgets":[{"category_id":` + itoa(groceriesID) + `,"amount":"100.00"},{"category_id":` + itoa(groceriesID) + `,"amount":"120.00"}]}`
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/budgets?month=2026-07", strings.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}
