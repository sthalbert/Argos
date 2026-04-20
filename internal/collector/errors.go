package collector

import (
	"errors"
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
// operation. It carries enough context for the single top-level handler to
// record metrics and emit a structured log line.
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

// pollErr is a convenience constructor.
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

// handlePollError is the single place where poll errors are handled:
// record Prometheus metric + emit structured log at the appropriate level.
func handlePollError(err error) {
	var pe *PollError
	if !errors.As(err, &pe) {
		slog.Error("collector: unexpected error", "error", err)
		return
	}

	metrics.ObserveError(pe.ClusterName, pe.Resource, pe.Operation)

	attrs := make([]any, 0, 4+len(pe.Attrs)*2)
	attrs = append(attrs, "error", pe.Err, "cluster_name", pe.ClusterName)
	for _, a := range pe.Attrs {
		attrs = append(attrs, a.Key, a.Value)
	}

	msg := fmt.Sprintf("collector: %s %s failed", pe.Operation, pe.Resource)
	switch pe.Severity {
	case SeverityError:
		slog.Error(msg, attrs...)
	default:
		slog.Warn(msg, attrs...)
	}
}
