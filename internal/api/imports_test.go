package api

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func venmoMultipart(t *testing.T, field string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile(field, "venmo.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, mw.FormDataContentType()
}

func TestVenmoUpload(t *testing.T) {
	s := testStore(t)
	srv := New(s)

	csv, err := os.ReadFile("../venmo/testdata/venmo-2026.csv")
	if err != nil {
		t.Fatal(err)
	}
	buf, ctype := venmoMultipart(t, "file", csv)
	req := httptest.NewRequest(http.MethodPost, "/api/imports/venmo", buf)
	req.Header.Set("Content-Type", ctype)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "upserted") {
		t.Fatalf("body missing upserted count: %s", rec.Body.String())
	}
}

func TestVenmoUploadMissingFile(t *testing.T) {
	srv := New(testStore(t))
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/imports/venmo", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}

func TestVenmoUploadBadCSV(t *testing.T) {
	srv := New(testStore(t))
	buf, ctype := venmoMultipart(t, "file", []byte("this,is,not\na,venmo,export\n"))
	req := httptest.NewRequest(http.MethodPost, "/api/imports/venmo", buf)
	req.Header.Set("Content-Type", ctype)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400", rec.Code, rec.Body.String())
	}
}

func TestVenmoUploadNotMultipart(t *testing.T) {
	srv := New(testStore(t))
	req := httptest.NewRequest(http.MethodPost, "/api/imports/venmo", strings.NewReader("plain body"))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}
