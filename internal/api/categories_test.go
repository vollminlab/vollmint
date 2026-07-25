package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListCategories(t *testing.T) {
	srv := New(testStore(t))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/categories", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var body struct {
		Categories []struct {
			Name string `json:"name"`
		} `json:"categories"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Categories) == 0 {
		t.Fatal("want seed categories")
	}
}

func TestCreateAndPatchCategory(t *testing.T) {
	srv := New(testStore(t))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/categories",
		strings.NewReader(`{"name":"Pets","kind":"spend","is_vice":false}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID int `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.ID == 0 {
		t.Fatal("no id returned")
	}
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/api/categories/"+itoa(created.ID),
		strings.NewReader(`{"is_vice":true}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status=%d", rec.Code)
	}
}

func TestCreateCategoryBadKind(t *testing.T) {
	srv := New(testStore(t))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/categories",
		strings.NewReader(`{"name":"X","kind":"bogus"}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}
