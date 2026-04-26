package provider

import "context"

// Fake is a deterministic Provider for tests. Returns the VMs slice
// verbatim and surfaces ListErr on demand. Safe for concurrent use only
// when the tests don't mutate the slice between calls.
type Fake struct {
	VMs     []VM
	ListErr error
	calls   int
}

// Kind returns the static label "fake".
func (f *Fake) Kind() string { return "fake" }

// ListVMs returns the configured VMs slice or the configured error.
func (f *Fake) ListVMs(_ context.Context) ([]VM, error) {
	f.calls++
	if f.ListErr != nil {
		return nil, f.ListErr
	}
	out := make([]VM, len(f.VMs))
	copy(out, f.VMs)
	return out, nil
}

// Calls returns how many times ListVMs has been called — useful in
// tests asserting on the collector tick cadence.
func (f *Fake) Calls() int { return f.calls }
