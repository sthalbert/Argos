package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// memStore is an in-memory api.Store implementation used to exercise the
// HTTP handlers without a PostgreSQL dependency.
type memStore struct {
	mu       sync.Mutex
	byID     map[uuid.UUID]Cluster
	byName   map[string]uuid.UUID
	pingErr  error
	createdN int
}

func newMemStore() *memStore {
	return &memStore{
		byID:   make(map[uuid.UUID]Cluster),
		byName: make(map[string]uuid.UUID),
	}
}

func (m *memStore) Ping(_ context.Context) error { return m.pingErr }

func (m *memStore) CreateCluster(_ context.Context, in ClusterCreate) (Cluster, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.byName[in.Name]; exists {
		return Cluster{}, fmt.Errorf("duplicate: %w", ErrConflict)
	}
	id := uuid.New()
	now := time.Now().UTC().Add(time.Duration(m.createdN) * time.Nanosecond)
	m.createdN++
	c := Cluster{
		Id:                &id,
		Name:              in.Name,
		DisplayName:       in.DisplayName,
		Environment:       in.Environment,
		Provider:          in.Provider,
		Region:            in.Region,
		KubernetesVersion: in.KubernetesVersion,
		ApiEndpoint:       in.ApiEndpoint,
		Labels:            in.Labels,
		CreatedAt:         &now,
		UpdatedAt:         &now,
	}
	m.byID[id] = c
	m.byName[in.Name] = id
	return c, nil
}

func (m *memStore) GetCluster(_ context.Context, id uuid.UUID) (Cluster, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.byID[id]
	if !ok {
		return Cluster{}, ErrNotFound
	}
	return c, nil
}

func (m *memStore) ListClusters(_ context.Context, limit int, _ string) ([]Cluster, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit <= 0 {
		limit = 50
	}
	out := make([]Cluster, 0, len(m.byID))
	for _, c := range m.byID {
		out = append(out, c)
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, "", nil
}

func (m *memStore) UpdateCluster(_ context.Context, id uuid.UUID, in ClusterUpdate) (Cluster, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.byID[id]
	if !ok {
		return Cluster{}, ErrNotFound
	}
	if in.DisplayName != nil {
		c.DisplayName = in.DisplayName
	}
	if in.Environment != nil {
		c.Environment = in.Environment
	}
	if in.Provider != nil {
		c.Provider = in.Provider
	}
	if in.Region != nil {
		c.Region = in.Region
	}
	if in.KubernetesVersion != nil {
		c.KubernetesVersion = in.KubernetesVersion
	}
	if in.ApiEndpoint != nil {
		c.ApiEndpoint = in.ApiEndpoint
	}
	if in.Labels != nil {
		c.Labels = in.Labels
	}
	now := time.Now().UTC()
	c.UpdatedAt = &now
	m.byID[id] = c
	return c, nil
}

func (m *memStore) DeleteCluster(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.byID[id]
	if !ok {
		return ErrNotFound
	}
	delete(m.byID, id)
	delete(m.byName, c.Name)
	return nil
}

func newTestHandler(t *testing.T, store Store) http.Handler {
	t.Helper()
	return Handler(NewServer("test", store))
}

func TestHealthAndReadiness(t *testing.T) {
	t.Parallel()

	t.Run("healthz ok", func(t *testing.T) {
		t.Parallel()
		h := newTestHandler(t, newMemStore())
		rr := do(h, http.MethodGet, "/healthz", "")
		if rr.Code != http.StatusOK {
			t.Fatalf("status=%d", rr.Code)
		}
		var got Health
		if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.Status != Ok {
			t.Errorf("status = %q", got.Status)
		}
	})

	t.Run("readyz ok when store pings", func(t *testing.T) {
		t.Parallel()
		h := newTestHandler(t, newMemStore())
		rr := do(h, http.MethodGet, "/readyz", "")
		if rr.Code != http.StatusOK {
			t.Fatalf("status=%d body=%q", rr.Code, rr.Body.String())
		}
	})

	t.Run("readyz 503 when store ping fails", func(t *testing.T) {
		t.Parallel()
		m := newMemStore()
		m.pingErr = errors.New("db down")
		h := newTestHandler(t, m)
		rr := do(h, http.MethodGet, "/readyz", "")
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%q", rr.Code, rr.Body.String())
		}
		if ct := rr.Header().Get("Content-Type"); ct != "application/problem+json" {
			t.Errorf("Content-Type=%q", ct)
		}
	})
}

func TestClusterCRUD(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t, newMemStore())

	// Create
	create := do(h, http.MethodPost, "/v1/clusters", `{"name":"prod-eu-west-1","environment":"prod"}`)
	if create.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%q", create.Code, create.Body.String())
	}
	if loc := create.Header().Get("Location"); !strings.HasPrefix(loc, "/v1/clusters/") {
		t.Errorf("Location=%q", loc)
	}
	var created Cluster
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.Id == nil {
		t.Fatal("created.Id is nil")
	}

	// Duplicate create → 409
	dup := do(h, http.MethodPost, "/v1/clusters", `{"name":"prod-eu-west-1"}`)
	if dup.Code != http.StatusConflict {
		t.Errorf("duplicate create status=%d", dup.Code)
	}

	// Get
	getURL := "/v1/clusters/" + created.Id.String()
	get := do(h, http.MethodGet, getURL, "")
	if get.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%q", get.Code, get.Body.String())
	}

	// Get missing → 404
	miss := do(h, http.MethodGet, "/v1/clusters/"+uuid.Nil.String(), "")
	if miss.Code != http.StatusNotFound {
		t.Errorf("get missing status=%d", miss.Code)
	}

	// Patch
	patch := do(h, http.MethodPatch, getURL, `{"provider":"gke"}`)
	if patch.Code != http.StatusOK {
		t.Fatalf("patch status=%d body=%q", patch.Code, patch.Body.String())
	}
	var patched Cluster
	if err := json.Unmarshal(patch.Body.Bytes(), &patched); err != nil {
		t.Fatalf("decode patch: %v", err)
	}
	if patched.Provider == nil || *patched.Provider != "gke" {
		t.Errorf("provider=%v", patched.Provider)
	}

	// List
	list := do(h, http.MethodGet, "/v1/clusters", "")
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d", list.Code)
	}
	var page ClusterList
	if err := json.Unmarshal(list.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(page.Items) != 1 {
		t.Errorf("list len=%d", len(page.Items))
	}

	// Delete
	del := do(h, http.MethodDelete, getURL, "")
	if del.Code != http.StatusNoContent {
		t.Errorf("delete status=%d", del.Code)
	}

	// Delete again → 404
	del2 := do(h, http.MethodDelete, getURL, "")
	if del2.Code != http.StatusNotFound {
		t.Errorf("second delete status=%d", del2.Code)
	}
}

func TestCreateClusterValidation(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t, newMemStore())

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"empty body", "", http.StatusBadRequest},
		{"missing name", `{"environment":"dev"}`, http.StatusBadRequest},
		{"unknown field", `{"name":"x","bogus":true}`, http.StatusBadRequest},
		{"malformed json", `{`, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rr := do(h, http.MethodPost, "/v1/clusters", tt.body)
			if rr.Code != tt.wantStatus {
				t.Errorf("status=%d want=%d body=%q", rr.Code, tt.wantStatus, rr.Body.String())
			}
		})
	}
}

func TestUnknownRoute404(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t, newMemStore())
	rr := do(h, http.MethodGet, "/no-such-path", "")
	if rr.Code != http.StatusNotFound {
		t.Errorf("status=%d", rr.Code)
	}
}

func do(h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}
