package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsEndpoint(t *testing.T) {
	srv := New(nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics status=%d", rec.Code)
	}
	// default Go collector always exposes this
	if !strings.Contains(rec.Body.String(), "go_goroutines") {
		t.Fatalf("metrics body missing go_goroutines")
	}
}

func TestSPAIndexServed(t *testing.T) {
	srv := New(nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "vollmint") {
		t.Fatalf("index status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSPAClientRouteFallsBackToIndex(t *testing.T) {
	srv := New(nil)
	rec := httptest.NewRecorder()
	// a client-side route that is not a real file must return index.html, not 404
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/transactions", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "vollmint") {
		t.Fatalf("client route status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUnknownAPIStill404(t *testing.T) {
	srv := New(nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/does-not-exist", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown api status=%d, want 404 (never SPA fallback)", rec.Code)
	}
}

func TestBareAPIPathStill404(t *testing.T) {
	srv := New(nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("bare /api status=%d, want 404 (never SPA fallback)", rec.Code)
	}
}
