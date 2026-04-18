package api

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// Sentinel errors returned by Store implementations. Handlers translate these
// into RFC 7807 responses with the matching HTTP status.
var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

// Store is the persistence contract consumed by the REST handlers.
// Implementations must be safe for concurrent use by multiple goroutines.
type Store interface {
	// Ping verifies that the underlying database is reachable.
	Ping(ctx context.Context) error

	// CreateCluster inserts a new cluster. Returns ErrConflict if a cluster
	// with the same name already exists.
	CreateCluster(ctx context.Context, in ClusterCreate) (Cluster, error)

	// GetCluster fetches a cluster by id. Returns ErrNotFound if absent.
	GetCluster(ctx context.Context, id uuid.UUID) (Cluster, error)

	// ListClusters returns up to limit clusters after the given opaque cursor,
	// plus the cursor for the next page (empty when exhausted).
	ListClusters(ctx context.Context, limit int, cursor string) (items []Cluster, nextCursor string, err error)

	// UpdateCluster applies the merge-patch fields set in in. Returns
	// ErrNotFound if the cluster does not exist.
	UpdateCluster(ctx context.Context, id uuid.UUID, in ClusterUpdate) (Cluster, error)

	// DeleteCluster removes a cluster by id. Returns ErrNotFound if absent.
	DeleteCluster(ctx context.Context, id uuid.UUID) error
}
