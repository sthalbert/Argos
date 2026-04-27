package auth

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestHasScope_AdminImpliesRegularScopes(t *testing.T) {
	t.Parallel()
	c := &Caller{Scopes: []string{ScopeAdmin}}
	for _, want := range []string{ScopeRead, ScopeWrite, ScopeDelete, ScopeAudit, ScopeAdmin} {
		if !c.HasScope(want) {
			t.Errorf("admin token should satisfy %q", want)
		}
	}
}

// Per ADR-0015 §5: admin scope must NOT imply vm-collector. Letting
// admin tokens fetch cloud-provider credentials would defeat the
// "SK is write-only from admin endpoints" guarantee.
func TestHasScope_AdminDoesNotImplyVMCollector(t *testing.T) {
	t.Parallel()
	c := &Caller{Scopes: []string{ScopeAdmin}}
	if c.HasScope(ScopeVMCollector) {
		t.Error("admin scope must not imply vm-collector")
	}
}

func TestHasScope_ExactMatch(t *testing.T) {
	t.Parallel()
	c := &Caller{Scopes: []string{ScopeVMCollector}}
	if !c.HasScope(ScopeVMCollector) {
		t.Error("vm-collector scope must match itself")
	}
	if c.HasScope(ScopeAdmin) {
		t.Error("vm-collector scope must not imply admin")
	}
	if c.HasScope(ScopeRead) {
		t.Error("vm-collector scope must not imply read")
	}
}

func TestHasScope_EditorMatches(t *testing.T) {
	t.Parallel()
	c := &Caller{Scopes: ScopesForRole(RoleEditor)}
	if !c.HasScope(ScopeRead) {
		t.Error("editor must satisfy read")
	}
	if !c.HasScope(ScopeWrite) {
		t.Error("editor must satisfy write")
	}
	if c.HasScope(ScopeDelete) {
		t.Error("editor must not satisfy delete")
	}
	if c.HasScope(ScopeVMCollector) {
		t.Error("editor must not satisfy vm-collector")
	}
}

func TestEnforceCloudAccountBinding_NonCollectorTokenPassesThrough(t *testing.T) {
	t.Parallel()
	c := &Caller{Scopes: []string{ScopeRead}}
	if err := c.EnforceCloudAccountBinding(uuid.New()); err != nil {
		t.Errorf("non-collector token should pass any binding check: %v", err)
	}
}

func TestEnforceCloudAccountBinding_CollectorWithNoBindingFails(t *testing.T) {
	t.Parallel()
	c := &Caller{
		Scopes:              []string{ScopeVMCollector},
		BoundCloudAccountID: nil,
	}
	err := c.EnforceCloudAccountBinding(uuid.New())
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("got %v, want ErrUnauthorized", err)
	}
}

func TestEnforceCloudAccountBinding_CollectorWithMatchingBindingPasses(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	c := &Caller{
		Scopes:              []string{ScopeVMCollector},
		BoundCloudAccountID: &id,
	}
	if err := c.EnforceCloudAccountBinding(id); err != nil {
		t.Errorf("matching binding must pass: %v", err)
	}
}

func TestEnforceCloudAccountBinding_CollectorWithMismatchedBindingFails(t *testing.T) {
	t.Parallel()
	bound := uuid.New()
	other := uuid.New()
	c := &Caller{
		Scopes:              []string{ScopeVMCollector},
		BoundCloudAccountID: &bound,
	}
	err := c.EnforceCloudAccountBinding(other)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("got %v, want ErrUnauthorized", err)
	}
}

func TestScopesForTokenPreset(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		preset string
		want   []string
	}{
		{TokenPresetAdmin, []string{ScopeRead, ScopeWrite, ScopeDelete, ScopeAdmin, ScopeAudit}},
		{TokenPresetEditor, []string{ScopeRead, ScopeWrite}},
		{TokenPresetAuditor, []string{ScopeRead, ScopeAudit}},
		{TokenPresetViewer, []string{ScopeRead}},
		{TokenPresetVMCollector, []string{ScopeVMCollector}},
	} {
		got := ScopesForTokenPreset(tc.preset)
		if !equalScopes(got, tc.want) {
			t.Errorf("ScopesForTokenPreset(%q) = %v, want %v", tc.preset, got, tc.want)
		}
	}
}

func equalScopes(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]struct{}, len(a))
	for _, s := range a {
		seen[s] = struct{}{}
	}
	for _, s := range b {
		if _, ok := seen[s]; !ok {
			return false
		}
	}
	return true
}
