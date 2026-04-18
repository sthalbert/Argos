package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
)

// Server implements ServerInterface for the Argos REST API.
type Server struct {
	version string
	store   Store
}

// NewServer wires the handlers with a persistence backend and the build
// version reported on health probes.
func NewServer(version string, store Store) *Server {
	return &Server{version: version, store: store}
}

var _ ServerInterface = (*Server)(nil)

// GetHealthz reports that the process is alive.
func (s *Server) GetHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, Health{Status: Ok, Version: &s.version})
}

// GetReadyz reports whether the service can accept traffic by pinging the store.
func (s *Server) GetReadyz(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		slog.Error("readyz: store ping failed", "error", err)
		writeProblem(w, http.StatusServiceUnavailable, "Not Ready", "database not reachable")
		return
	}
	writeJSON(w, http.StatusOK, Health{Status: Ok, Version: &s.version})
}

// ListClusters returns a paged list of clusters.
func (s *Server) ListClusters(w http.ResponseWriter, r *http.Request, params ListClustersParams) {
	limit := 0
	if params.Limit != nil {
		limit = *params.Limit
	}
	cursor := ""
	if params.Cursor != nil {
		cursor = *params.Cursor
	}

	items, next, err := s.store.ListClusters(r.Context(), limit, cursor)
	if err != nil {
		s.writeStoreError(w, "listClusters", err)
		return
	}

	resp := ClusterList{Items: items}
	if next != "" {
		resp.NextCursor = &next
	}
	writeJSON(w, http.StatusOK, resp)
}

// CreateCluster registers a new cluster.
func (s *Server) CreateCluster(w http.ResponseWriter, r *http.Request) {
	var body ClusterCreate
	if err := decodeJSONBody(r, &body); err != nil {
		writeProblem(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	if body.Name == "" {
		writeProblem(w, http.StatusBadRequest, "Missing field", "field 'name' is required")
		return
	}

	c, err := s.store.CreateCluster(r.Context(), body)
	if err != nil {
		s.writeStoreError(w, "createCluster", err)
		return
	}

	if c.Id != nil {
		w.Header().Set("Location", "/v1/clusters/"+c.Id.String())
	}
	writeJSON(w, http.StatusCreated, c)
}

// GetCluster fetches a cluster by id.
func (s *Server) GetCluster(w http.ResponseWriter, r *http.Request, id ClusterId) {
	c, err := s.store.GetCluster(r.Context(), id)
	if err != nil {
		s.writeStoreError(w, "getCluster", err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// UpdateCluster applies merge-patch updates to a cluster.
func (s *Server) UpdateCluster(w http.ResponseWriter, r *http.Request, id ClusterId) {
	var body ClusterUpdate
	if err := decodeJSONBody(r, &body); err != nil {
		writeProblem(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	c, err := s.store.UpdateCluster(r.Context(), id, body)
	if err != nil {
		s.writeStoreError(w, "updateCluster", err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// DeleteCluster removes a cluster.
func (s *Server) DeleteCluster(w http.ResponseWriter, r *http.Request, id ClusterId) {
	if err := s.store.DeleteCluster(r.Context(), id); err != nil {
		s.writeStoreError(w, "deleteCluster", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) writeStoreError(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeProblem(w, http.StatusNotFound, "Not Found", "")
	case errors.Is(err, ErrConflict):
		writeProblem(w, http.StatusConflict, "Conflict", err.Error())
	default:
		slog.Error("handler store error", "op", op, "error", err)
		writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "")
	}
}

func decodeJSONBody(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("request body is empty")
		}
		return err
	}
	return nil
}
