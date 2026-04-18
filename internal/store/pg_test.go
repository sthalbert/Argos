package store

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"

	"github.com/google/uuid"

	"github.com/sthalbert/argos/internal/api"
)

// newTestPG returns a PG connected to PGX_TEST_DATABASE, or calls t.Skip
// when the env var is unset. Every test runs against a freshly migrated
// schema and is cleaned up with TRUNCATE on t.Cleanup.
func newTestPG(t *testing.T) *PG {
	t.Helper()
	dsn := os.Getenv("PGX_TEST_DATABASE")
	if dsn == "" {
		t.Skip("PGX_TEST_DATABASE not set; skipping integration test")
	}

	ctx := context.Background()
	pg, err := Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := pg.Migrate(ctx); err != nil {
		pg.Close()
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pg.pool.Exec(context.Background(), "TRUNCATE clusters")
		pg.Close()
	})
	return pg
}

func TestPGClusterCRUD(t *testing.T) {
	pg := newTestPG(t)
	ctx := context.Background()

	name := "test-" + strconv.FormatInt(int64(uuid.New().ID()), 16)
	env := "staging"
	created, err := pg.CreateCluster(ctx, api.ClusterCreate{
		Name:        name,
		Environment: &env,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Id == nil {
		t.Fatal("created.Id is nil")
	}

	got, err := pg.GetCluster(ctx, *created.Id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != name {
		t.Errorf("name = %q, want %q", got.Name, name)
	}
	if got.Environment == nil || *got.Environment != env {
		t.Errorf("environment = %v, want %q", got.Environment, env)
	}

	_, err = pg.CreateCluster(ctx, api.ClusterCreate{Name: name})
	if !errors.Is(err, api.ErrConflict) {
		t.Errorf("duplicate should be ErrConflict, got %v", err)
	}

	prov := "gke"
	updated, err := pg.UpdateCluster(ctx, *created.Id, api.ClusterUpdate{Provider: &prov})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Provider == nil || *updated.Provider != prov {
		t.Errorf("provider after update = %v, want %q", updated.Provider, prov)
	}

	if err := pg.DeleteCluster(ctx, *created.Id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := pg.DeleteCluster(ctx, *created.Id); !errors.Is(err, api.ErrNotFound) {
		t.Errorf("second delete should be ErrNotFound, got %v", err)
	}
	if _, err := pg.GetCluster(ctx, *created.Id); !errors.Is(err, api.ErrNotFound) {
		t.Errorf("get after delete should be ErrNotFound, got %v", err)
	}
}

func TestPGListPagination(t *testing.T) {
	pg := newTestPG(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		name := "page-" + strconv.Itoa(i)
		if _, err := pg.CreateCluster(ctx, api.ClusterCreate{Name: name}); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}

	page1, next, err := pg.ListClusters(ctx, 2, "")
	if err != nil {
		t.Fatalf("list page1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("page1 len=%d, want 2", len(page1))
	}
	if next == "" {
		t.Fatal("next cursor empty after page1")
	}

	page2, next, err := pg.ListClusters(ctx, 2, next)
	if err != nil {
		t.Fatalf("list page2: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("page2 len=%d, want 2", len(page2))
	}

	page3, next, err := pg.ListClusters(ctx, 2, next)
	if err != nil {
		t.Fatalf("list page3: %v", err)
	}
	if len(page3) != 1 {
		t.Fatalf("page3 len=%d, want 1", len(page3))
	}
	if next != "" {
		t.Errorf("next should be empty on last page, got %q", next)
	}

	seen := make(map[uuid.UUID]bool)
	for _, c := range append(append(page1, page2...), page3...) {
		if c.Id == nil {
			t.Fatal("cluster id nil")
		}
		if seen[*c.Id] {
			t.Errorf("duplicate id %v across pages", *c.Id)
		}
		seen[*c.Id] = true
	}
}
