package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateRuleAppliesToHistory(t *testing.T) {
	s := testStore(t)
	// an uncategorized CHIPOTLE charge already in history
	seedTxn(t, s, "ally-s", "scott", "c1", "2026-07-05", "-12.00", "CHIPOTLE 4021")
	srv := New(s)

	var diningID int
	_ = s.Pool.QueryRow(context.Background(), `SELECT id FROM categories WHERE name='Dining'`).Scan(&diningID)

	rec := httptest.NewRecorder()
	body := `{"priority":10,"match_type":"substring","pattern":"CHIPOTLE","category_id":` + itoa(diningID) + `}`
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/rules", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID            int `json:"id"`
		Recategorized int `json:"recategorized"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Recategorized != 1 {
		t.Fatalf("recategorized=%d, want 1 (rule should re-run over history)", created.Recategorized)
	}
	// the historical charge is now Dining
	var name string
	_ = s.Pool.QueryRow(context.Background(),
		`SELECT c.name FROM transactions t JOIN categories c ON c.id=t.category_id WHERE t.external_id='c1'`).Scan(&name)
	if name != "Dining" {
		t.Fatalf("charge category=%q, want Dining", name)
	}
}

func TestDeleteRule(t *testing.T) {
	s := testStore(t)
	srv := New(s)
	var diningID int
	_ = s.Pool.QueryRow(context.Background(), `SELECT id FROM categories WHERE name='Dining'`).Scan(&diningID)
	id, _ := s.CreateRule(context.Background(), 10, "substring", "X", diningID)

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/rules/"+itoa(id), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status=%d", rec.Code)
	}
}

func TestListRules(t *testing.T) {
	s := testStore(t)
	srv := New(s)
	var diningID int
	_ = s.Pool.QueryRow(context.Background(), `SELECT id FROM categories WHERE name='Dining'`).Scan(&diningID)
	s.CreateRule(context.Background(), 10, "substring", "TEST", diningID)

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/rules", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var body struct {
		Rules []struct{} `json:"rules"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Rules == nil {
		t.Fatal("rules array missing in response")
	}
}

func TestCreateRuleBadJSON(t *testing.T) {
	srv := New(testStore(t))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/rules", strings.NewReader(`{`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}

func TestCreateRuleMissingFields(t *testing.T) {
	srv := New(testStore(t))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/rules", strings.NewReader(`{"priority":1}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}

func TestCreateRuleBadMatchType(t *testing.T) {
	s := testStore(t)
	srv := New(s)
	var diningID int
	_ = s.Pool.QueryRow(context.Background(), `SELECT id FROM categories WHERE name='Dining'`).Scan(&diningID)

	rec := httptest.NewRecorder()
	body := `{"pattern":"X","category_id":` + itoa(diningID) + `,"match_type":"glob"}`
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/rules", strings.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}

func TestCreateRuleInvalidRegex(t *testing.T) {
	s := testStore(t)
	srv := New(s)
	var diningID int
	_ = s.Pool.QueryRow(context.Background(), `SELECT id FROM categories WHERE name='Dining'`).Scan(&diningID)

	rec := httptest.NewRecorder()
	body := `{"pattern":"[unclosed","category_id":` + itoa(diningID) + `,"match_type":"regex"}`
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/rules", strings.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}

func TestCreateRuleBadCategoryFK(t *testing.T) {
	srv := New(testStore(t))
	rec := httptest.NewRecorder()
	body := `{"pattern":"X","category_id":999999}`
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/rules", strings.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}

func TestDeleteRuleBadID(t *testing.T) {
	srv := New(testStore(t))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/rules/abc", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}

func TestDeleteRuleNotFound(t *testing.T) {
	srv := New(testStore(t))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/rules/999999", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", rec.Code)
	}
}
