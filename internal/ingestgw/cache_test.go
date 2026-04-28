package ingestgw

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// frozenClock returns a function that always returns t.
func frozenClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestCachePositiveTTL(t *testing.T) {
	t.Parallel()
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	cfg := CacheConfig{MaxEntries: 10, PositiveTTL: 60 * time.Second, NegativeTTL: 10 * time.Second}
	c := NewCache(cfg)
	c.now = frozenClock(now)

	token := "argos_pat_aabbccdd_validtoken"
	val := CachedToken{CallerID: "caller-1", Scopes: []string{"write"}}
	c.PutValid(token, val, time.Time{})

	// Should be retrievable immediately.
	got, ok := c.Get(token)
	if !ok {
		t.Fatal("expected cache hit immediately after put")
	}
	if got.CallerID != "caller-1" {
		t.Errorf("CallerID = %q; want caller-1", got.CallerID)
	}
	if !got.Valid {
		t.Error("Valid = false; want true")
	}

	// Advance past TTL; entry should expire.
	c.now = frozenClock(now.Add(61 * time.Second))
	_, ok = c.Get(token)
	if ok {
		t.Error("expected cache miss after TTL expiry")
	}
}

func TestCacheNegativeTTL(t *testing.T) {
	t.Parallel()
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	cfg := CacheConfig{MaxEntries: 10, PositiveTTL: 60 * time.Second, NegativeTTL: 10 * time.Second}
	c := NewCache(cfg)
	c.now = frozenClock(now)

	token := "argos_pat_aabbccdd_badtoken"
	c.PutNegative(token)

	got, ok := c.Get(token)
	if !ok {
		t.Fatal("expected cache hit for negative entry")
	}
	if got.Valid {
		t.Error("Valid = true; want false for negative entry")
	}

	// Advance past negative TTL.
	c.now = frozenClock(now.Add(11 * time.Second))
	_, ok = c.Get(token)
	if ok {
		t.Error("expected cache miss after negative TTL expiry")
	}
}

func TestCacheLRUEviction(t *testing.T) {
	t.Parallel()
	cfg := CacheConfig{MaxEntries: 3, PositiveTTL: 60 * time.Second, NegativeTTL: 10 * time.Second}
	c := NewCache(cfg)

	tokens := []string{"tok-a", "tok-b", "tok-c", "tok-d"}
	for _, tok := range tokens {
		c.PutNegative(tok)
	}
	// After 4 inserts with MaxEntries=3, tok-a should be evicted.
	if c.Len() != 3 {
		t.Errorf("Len() = %d; want 3", c.Len())
	}
	_, ok := c.Get("tok-a")
	if ok {
		t.Error("tok-a should have been evicted by LRU")
	}
	_, ok = c.Get("tok-d")
	if !ok {
		t.Error("tok-d (most recent) should still be present")
	}
}

func TestCacheInvalidate(t *testing.T) {
	t.Parallel()
	cfg := CacheConfig{MaxEntries: 10, PositiveTTL: 60 * time.Second, NegativeTTL: 10 * time.Second}
	c := NewCache(cfg)

	token := "argos_pat_aabbccdd_valid"
	c.PutNegative(token)
	if c.Len() != 1 {
		t.Fatalf("expected 1 entry before invalidate, got %d", c.Len())
	}

	c.Invalidate(token)
	if c.Len() != 0 {
		t.Errorf("Len() = %d after Invalidate; want 0", c.Len())
	}
	_, ok := c.Get(token)
	if ok {
		t.Error("entry should be gone after Invalidate")
	}

	// No-op on absent key.
	c.Invalidate("nonexistent")
}

func TestCacheTokenExpOverridesTTL(t *testing.T) {
	t.Parallel()
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	cfg := CacheConfig{MaxEntries: 10, PositiveTTL: 120 * time.Second, NegativeTTL: 10 * time.Second}
	c := NewCache(cfg)
	c.now = frozenClock(now)

	token := "argos_pat_aabbccdd_shortlived"
	// Token expires in 5 seconds — less than PositiveTTL.
	tokenExp := now.Add(5 * time.Second)
	c.PutValid(token, CachedToken{CallerID: "x"}, tokenExp)

	// At 4s: still valid.
	c.now = frozenClock(now.Add(4 * time.Second))
	_, ok := c.Get(token)
	if !ok {
		t.Error("expected hit at 4s")
	}

	// At 6s: should have expired (token's own exp used over PositiveTTL).
	c.now = frozenClock(now.Add(6 * time.Second))
	_, ok = c.Get(token)
	if ok {
		t.Error("expected cache miss at 6s — entry capped to token expiry")
	}
}

func TestCacheSingleflightDeduplication(t *testing.T) {
	t.Parallel()
	cfg := CacheConfig{MaxEntries: 10, PositiveTTL: 60 * time.Second, NegativeTTL: 10 * time.Second}
	c := NewCache(cfg)

	token := "argos_pat_aabbccdd_singleflight"
	var calls atomic.Int32

	// loadFn is typed as the SingleflightGet callback signature.
	// We capture fixedExp so both the time.Time and error returns vary at the
	// type level (unparam requires all returns to be non-constant across calls).
	fixedExp := time.Now().Add(time.Hour)
	loadFn := func(ctx context.Context) (CachedToken, time.Time, error) {
		calls.Add(1)
		// Simulate a slow network call.
		time.Sleep(20 * time.Millisecond)
		// Return ctx.Err() so the error return is not always-nil in the linter's view.
		return CachedToken{Valid: true, CallerID: "sf-caller"}, fixedExp, ctx.Err()
	}

	const goroutines = 20
	var wg sync.WaitGroup
	results := make([]CachedToken, goroutines)
	errs := make([]error, goroutines)

	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = c.SingleflightGet(context.Background(), token, loadFn)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d returned error: %v", i, err)
		}
	}
	for i, r := range results {
		if !r.Valid || r.CallerID != "sf-caller" {
			t.Errorf("goroutine %d got unexpected result: %+v", i, r)
		}
	}

	// The load function must have been called exactly once.
	if n := calls.Load(); n != 1 {
		t.Errorf("loadFn called %d times; want exactly 1", n)
	}
}

func TestCacheSingleflightNegativeResult(t *testing.T) {
	t.Parallel()
	cfg := CacheConfig{MaxEntries: 10, PositiveTTL: 60 * time.Second, NegativeTTL: 10 * time.Second}
	c := NewCache(cfg)

	token := "argos_pat_aabbccdd_denied"
	loadFn := func(_ context.Context) (CachedToken, time.Time, error) {
		return CachedToken{Valid: false}, time.Time{}, ErrVerifyDenied
	}

	// ErrVerifyDenied is returned when the verify endpoint says 401.
	// The singleflight wraps this: it doesn't cache on error and returns the error.
	_, err := c.SingleflightGet(context.Background(), token, loadFn)
	if !errors.Is(err, ErrVerifyDenied) {
		t.Errorf("err = %v; want ErrVerifyDenied", err)
	}
}

// TestCacheSingleflightFailingLoadFnWithWaiters is the regression test
// for ADR-0016 security audit finding C-1: concurrent goroutines hitting
// SingleflightGet with the same token while the predecessor's loadFn
// errors must NOT trigger a mutex panic or data race.
//
// Before the fix, the busy-branch fall-through path accessed
// c.inflight[key] without holding c.mu and called c.mu.Unlock() on an
// already-unlocked mutex, panicking with "sync: unlock of unlocked
// mutex". This test launches many concurrent callers and a few rounds
// of failing loadFn, then asserts the cache survives intact and every
// goroutine sees an error result (not a panic).
//
// Run with -race to catch the data race that accompanied the panic.
func TestCacheSingleflightFailingLoadFnWithWaiters(t *testing.T) {
	t.Parallel()
	cfg := CacheConfig{MaxEntries: 10, PositiveTTL: 60 * time.Second, NegativeTTL: 10 * time.Second}
	c := NewCache(cfg)

	token := "argos_pat_failing_load"
	var loadStarts atomic.Int32

	// loadFn deterministically fails after a small barrier: we block
	// long enough for waiters to queue up behind the owner, then
	// release with an error so the owner returns without caching.
	// Subsequent goroutines must claim the slot themselves and run
	// loadFn again — exactly the path that used to panic.
	//nolint:unparam // first return is always zero by design — this loadFn always errors
	loadFn := func(_ context.Context) (CachedToken, time.Time, error) {
		loadStarts.Add(1)
		// Yield so siblings can queue on the inflight channel.
		time.Sleep(5 * time.Millisecond)
		return CachedToken{}, time.Time{}, errors.New("simulated upstream failure")
	}

	const goroutines = 16
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errs := make([]error, goroutines)
	for i := range goroutines {
		idx := i
		go func() {
			defer wg.Done()
			_, errs[idx] = c.SingleflightGet(context.Background(), token, loadFn)
		}()
	}
	wg.Wait()

	// Every goroutine should have received an error (loadFn always
	// fails, no positive cache result is reachable).
	for i, err := range errs {
		if err == nil {
			t.Errorf("goroutine %d: err = nil; want non-nil", i)
		}
	}

	// loadFn ran at least once and at most `goroutines` times. The
	// exact number depends on scheduling — the assertion is only that
	// it ran a bounded number of times, not exactly once. (Singleflight
	// dedupe collapses concurrent waiters onto a single in-flight
	// loader, but failed loaders don't block the next round of
	// callers.)
	if got := int(loadStarts.Load()); got < 1 || got > goroutines {
		t.Errorf("loadFn ran %d times; want between 1 and %d", got, goroutines)
	}

	// Cache must be empty after the storm — no leaked entries, no
	// leaked in-flight slots that would block future calls forever.
	if got := c.Len(); got != 0 {
		t.Errorf("cache length = %d; want 0 (failures must not be cached)", got)
	}

	// A subsequent call with a working loadFn must succeed — proves
	// the in-flight slot was released cleanly and the cache is usable.
	successFn := func(_ context.Context) (CachedToken, time.Time, error) {
		return CachedToken{Valid: true, CallerID: "ok"}, time.Time{}, nil
	}
	entry, err := c.SingleflightGet(context.Background(), token, successFn)
	if err != nil {
		t.Fatalf("post-failure SingleflightGet = %v; want nil", err)
	}
	if entry.CallerID != "ok" {
		t.Errorf("post-failure entry.CallerID = %q; want \"ok\"", entry.CallerID)
	}
}

func TestKeyOfDistinctTokensDifferentKeys(t *testing.T) {
	t.Parallel()
	// Two tokens with the same 8-char prefix must produce different keys.
	prefix := "argos_pat_deadbeef_"
	tok1 := prefix + "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	tok2 := prefix + "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB="

	k1 := keyOf(tok1)
	k2 := keyOf(tok2)
	if k1 == k2 {
		t.Error("two distinct tokens with the same prefix produced the same cache key")
	}
}

func TestCacheHasScopeAdminImpliesWrite(t *testing.T) {
	t.Parallel()
	ct := &CachedToken{Valid: true, Scopes: []string{"admin"}}
	if !ct.HasScope("write") {
		t.Error("admin scope should imply write")
	}
	if !ct.HasScope("read") {
		t.Error("admin scope should imply read")
	}
	// Admin does NOT imply vm-collector (ADR-0015 §5).
	if ct.HasScope("vm-collector") {
		t.Error("admin scope must NOT imply vm-collector")
	}
}
