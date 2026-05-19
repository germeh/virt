package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"warm-migration-core/internal/host"
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

func TestServerHostCapabilitiesEndpoint(t *testing.T) {
	server := NewServer(WithHostService(fakeHostService{
		caps: host.Capabilities{
			Ready:         true,
			KVMDevice:     true,
			QEMU:          true,
			Libvirt:       true,
			VirtInstall:   true,
			OVMF:          true,
			LibvirtActive: true,
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/host/capabilities", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"ready":true`) {
		t.Fatalf("response body = %s", rec.Body.String())
	}
}

func TestServerHostVMsEndpoint(t *testing.T) {
	server := NewServer(WithHostService(fakeHostService{
		vms: []host.VM{{ID: "1", Name: "web-01", State: "running"}},
	}))
	req := httptest.NewRequest(http.MethodGet, "/host/vms", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"name":"web-01"`) {
		t.Fatalf("response body = %s", rec.Body.String())
	}
}

func TestServerHostEndpointsRequireConfiguredHostService(t *testing.T) {
	server := NewServer()
	req := httptest.NewRequest(http.MethodGet, "/host/capabilities", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"host_service_not_configured"`) {
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

type fakeHostService struct {
	caps host.Capabilities
	vms  []host.VM
}

func (f fakeHostService) Capabilities(context.Context) (host.Capabilities, error) {
	return f.caps, nil
}

func (f fakeHostService) ListVMs(context.Context) ([]host.VM, error) {
	return f.vms, nil
}
