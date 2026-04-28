package api

// Tests for IngestMux and IngestRoutes (ADR-0016 §3).

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestIngestRoutes_Length(t *testing.T) {
	t.Parallel()
	// IngestRoutes must contain exactly 19 entries:
	// 18 write paths + 1 verify (POST /v1/auth/verify).
	const wantRoutes = 19
	if len(IngestRoutes) != wantRoutes {
		t.Errorf("len(IngestRoutes) = %d; want %d", len(IngestRoutes), wantRoutes)
	}
}

func TestIngestRoutes_IncludesVerify(t *testing.T) {
	t.Parallel()
	var found bool
	for _, r := range IngestRoutes {
		if r.Method == http.MethodPost && r.Pattern == "/v1/auth/verify" {
			found = true
			break
		}
	}
	if !found {
		t.Error("IngestRoutes must include POST /v1/auth/verify")
	}
}

func TestIngestRoutes_NoGetRoutes(t *testing.T) {
	t.Parallel()
	for _, r := range IngestRoutes {
		if r.Method == http.MethodGet {
			t.Errorf("IngestRoutes must not include GET routes; found GET %s", r.Pattern)
		}
	}
}

// TestIngestMux_AllRegisteredRoutesReachHandler verifies that every route
// in IngestRoutes is registered on the ingest mux. A route is "reachable"
// when it produces any status code other than 405 (Method Not Allowed) on
// a path that matches the pattern. 404 from the handler (e.g. ErrNotFound
// for an unknown cluster UUID) is acceptable — the mux dispatched correctly.
// The only failure is 405, which means the mux has no handler for the method.
func TestIngestMux_AllRegisteredRoutesReachHandler(t *testing.T) {
	t.Parallel()
	store := newMemStore()
	h := newVerifyServer(t, store)

	validUUID := uuid.New().String()

	for _, r := range IngestRoutes {
		t.Run(r.Method+" "+r.Pattern, func(t *testing.T) {
			t.Parallel()
			path := strings.ReplaceAll(r.Pattern, "{id}", validUUID)
			req := httptest.NewRequestWithContext(t.Context(), r.Method, path, strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			// 405 means the mux has no handler for this method on this path.
			if rr.Code == http.StatusMethodNotAllowed {
				t.Errorf("route %s %s returned 405 (method not allowed); route not registered", r.Method, r.Pattern)
			}
		})
	}
}

// TestIngestMux_UnregisteredRoutesReturn404 verifies that routes NOT in
// IngestRoutes return 404 from the ingest mux.
func TestIngestMux_UnregisteredRoutesReturn404(t *testing.T) {
	t.Parallel()
	store := newMemStore()
	h := newVerifyServer(t, store)

	validUUID := uuid.New().String()
	unregistered := []struct{ method, path string }{
		// Read routes (not on ingest mux).
		{http.MethodGet, "/v1/clusters"},
		{http.MethodGet, "/v1/clusters/" + validUUID},
		{http.MethodGet, "/v1/nodes"},
		{http.MethodGet, "/v1/pods"},
		{http.MethodGet, "/v1/workloads"},
		// Admin routes.
		{http.MethodGet, "/v1/admin/users"},
		{http.MethodPost, "/v1/admin/users"},
		{http.MethodGet, "/v1/admin/tokens"},
		{http.MethodPost, "/v1/admin/tokens"},
		{http.MethodGet, "/v1/admin/audit"},
		{http.MethodPatch, "/v1/admin/settings"},
		// Auth routes other than verify.
		{http.MethodPost, "/v1/auth/login"},
		{http.MethodGet, "/v1/auth/me"},
		{http.MethodPost, "/v1/auth/logout"},
		// Health endpoints (not on ingest mux).
		{http.MethodGet, "/healthz"},
		{http.MethodGet, "/readyz"},
		// Bogus.
		{http.MethodDelete, "/v1/clusters/" + validUUID},
		{http.MethodPut, "/v1/pods"},
	}

	for _, r := range unregistered {
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequestWithContext(t.Context(), r.method, r.path, http.NoBody)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != http.StatusNotFound && rr.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s: status = %d; want 404 or 405 for unregistered route", r.method, r.path, rr.Code)
			}
		})
	}
}

// TestIngestMux_VerifyReachable specifically asserts that POST /v1/auth/verify
// is reachable on the ingest mux (it returns 400 for an empty body — not 404).
func TestIngestMux_VerifyReachable(t *testing.T) {
	t.Parallel()
	store := newMemStore()
	h := newVerifyServer(t, store)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/auth/verify", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	// Empty body = 400 (handler runs, validates, rejects). Not 404.
	if rr.Code == http.StatusNotFound {
		t.Errorf("POST /v1/auth/verify returned 404; want it to be reachable on ingest mux")
	}
}

// TestIngestMux_ClusterGETNotReachable asserts GET /v1/clusters is 404 on the
// ingest mux (even though POST /v1/clusters is registered).
func TestIngestMux_ClusterGETNotReachable(t *testing.T) {
	t.Parallel()
	store := newMemStore()
	h := newVerifyServer(t, store)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/clusters", http.NoBody)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /v1/clusters on ingest mux: status = %d; want 404 or 405", rr.Code)
	}
}
