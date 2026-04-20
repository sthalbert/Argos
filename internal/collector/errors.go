package collector

import (
	"fmt"
	"log/slog"

	"github.com/sthalbert/argos/internal/metrics"
)

// Severity controls whether an error is logged as a warning (transient,
// expected) or an error (unexpected, likely bug or misconfiguration).
type Severity int

const (
	SeverityWarn  Severity = iota // transient / expected failures
	SeverityError                 // unexpected / likely config issue
)

// PollError represents a collector error tied to a specific resource and
// operation. It centralizes metric recording and structured logging so
// call sites don't repeat the observe+log boilerplate.
type PollError struct {
	// ClusterName identifies the target cluster.
	ClusterName string
	// Resource is the Kubernetes resource kind (e.g. "nodes", "pods").
	Resource string
	// Operation is the action that failed (e.g. "list", "upsert", "reconcile").
	Operation string
	// Err is the underlying error.
	Err error
	// Severity controls the log level.
	Severity Severity
	// Attrs are additional structured log attributes (e.g. node name).
	Attrs []slog.Attr
}

// Error implements the error interface.
func (e *PollError) Error() string {
	return fmt.Sprintf("collector: %s %s failed: %v", e.Operation, e.Resource, e.Err)
}

// Unwrap supports errors.Is/As.
func (e *PollError) Unwrap() error {
	return e.Err
}

// Report records the error in Prometheus metrics and emits a structured log
// line at the appropriate severity. Call this once at the error site; the
// caller retains control flow (return, continue, etc.).
func (e *PollError) Report() {
	metrics.ObserveError(e.ClusterName, e.Resource, e.Operation)

	attrs := make([]any, 0, 4+len(e.Attrs)*2)
	attrs = append(attrs, "error", e.Err, "cluster_name", e.ClusterName)
	for _, a := range e.Attrs {
		attrs = append(attrs, a.Key, a.Value)
	}

	msg := fmt.Sprintf("collector: %s %s failed", e.Operation, e.Resource)
	switch e.Severity {
	case SeverityError:
		slog.Error(msg, attrs...)
	default:
		slog.Warn(msg, attrs...)
	}
}

// pollErr is a convenience constructor for the common case.
func pollErr(clusterName, resource, operation string, err error, severity Severity, attrs ...slog.Attr) *PollError {
	return &PollError{
		ClusterName: clusterName,
		Resource:    resource,
		Operation:   operation,
		Err:         err,
		Severity:    severity,
		Attrs:       attrs,
	}
}
