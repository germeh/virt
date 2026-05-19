package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServerPreflightEndpoint(t *testing.T) {
	server := NewServer()
	req := httptest.NewRequest(http.MethodPost, "/migrations/preflight", strings.NewReader(`{"vm_id":"vm-123"}`))
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"ok":false`) {
		t.Fatalf("response body = %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"preflight_not_implemented"`) {
		t.Fatalf("response body = %s", rec.Body.String())
	}
}

func TestServerHealthEndpoint(t *testing.T) {
	server := NewServer()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Fatalf("response body = %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"service":"warm-migrationd"`) {
		t.Fatalf("response body = %s", rec.Body.String())
	}
}

func TestServerRejectsUnknownRoute(t *testing.T) {
	server := NewServer()
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestServerRejectsWrongPreflightMethod(t *testing.T) {
	server := NewServer()
	req := httptest.NewRequest(http.MethodGet, "/migrations/preflight", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
