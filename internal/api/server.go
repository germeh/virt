package api

import (
	"context"
	"encoding/json"
	"net/http"

	"warm-migration-core/internal/host"
)

type Server struct {
	mux  *http.ServeMux
	host HostService
}

type HostService interface {
	Capabilities(ctx context.Context) (host.Capabilities, error)
	ListVMs(ctx context.Context) ([]host.VM, error)
}

type Option func(*Server)

func WithHostService(hostService HostService) Option {
	return func(s *Server) {
		s.host = hostService
	}
}

func NewServer(options ...Option) *Server {
	s := &Server{mux: http.NewServeMux()}
	for _, option := range options {
		option(s)
	}
	s.mux.HandleFunc("/healthz", s.handleHealthz)
	s.mux.HandleFunc("/host/capabilities", s.handleHostCapabilities)
	s.mux.HandleFunc("/host/vms", s.handleHostVMs)
	s.mux.HandleFunc("/migrations/preflight", s.handlePreflight)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"service": "warm-migrationd",
	})
}

func (s *Server) handlePreflight(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusNotImplemented, map[string]any{
		"ok": false,
		"failures": []map[string]string{
			{
				"code":    "preflight_not_implemented",
				"message": "preflight endpoint is not wired to the compatibility checker yet",
			},
		},
	})
}

func (s *Server) handleHostCapabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.host == nil {
		writeHostNotConfigured(w)
		return
	}
	caps, err := s.host.Capabilities(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"capabilities": caps,
	})
}

func (s *Server) handleHostVMs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.host == nil {
		writeHostNotConfigured(w)
		return
	}
	vms, err := s.host.ListVMs(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":  true,
		"vms": vms,
	})
}

func writeHostNotConfigured(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotImplemented, map[string]any{
		"ok": false,
		"failures": []map[string]string{
			{
				"code":    "host_service_not_configured",
				"message": "host service is not configured for this API server",
			},
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
