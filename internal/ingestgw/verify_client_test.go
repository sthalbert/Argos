package ingestgw

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fakeVerifyHandler validates the request shape and returns a canned verify response.
func fakeVerifyHandler(expUnix int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "wrong method", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/v1/auth/verify" {
			http.Error(w, "wrong path", http.StatusNotFound)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var req struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(body, &req); err != nil || req.Token == "" {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(verifyResponseBody{
			Valid:     true,
			CallerID:  "caller-uuid-1234",
			Kind:      "token",
			TokenName: "my-collector-token",
			Scopes:    []string{"write"},
			Exp:       expUnix,
		})
	}
}

func TestVerifyClient_200_ValidToken(t *testing.T) {
	t.Parallel()

	expUnix := time.Now().Add(1 * time.Hour).Unix()
	fake := httptest.NewServer(fakeVerifyHandler(expUnix))
	t.Cleanup(fake.Close)

	vc := NewVerifyClient(fake.Client(), fake.URL)
	entry, exp, err := vc.Verify(t.Context(), "argos_pat_aabbccdd_validtoken")
	if err != nil {
		t.Fatalf("Verify() error = %v; want nil", err)
	}
	if !entry.Valid {
		t.Error("entry.Valid = false; want true")
	}
	if entry.CallerID != "caller-uuid-1234" {
		t.Errorf("CallerID = %q; want caller-uuid-1234", entry.CallerID)
	}
	if entry.TokenName != "my-collector-token" {
		t.Errorf("TokenName = %q; want my-collector-token", entry.TokenName)
	}
	if len(entry.Scopes) != 1 || entry.Scopes[0] != "write" {
		t.Errorf("Scopes = %v; want [write]", entry.Scopes)
	}
	if exp.Unix() != expUnix {
		t.Errorf("exp.Unix() = %d; want %d", exp.Unix(), expUnix)
	}
}

func TestVerifyClient_200_NoExpiry(t *testing.T) {
	t.Parallel()

	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(verifyResponseBody{
			Valid:  true,
			Kind:   "token",
			Scopes: []string{"read"},
			// Exp is 0 / omitted — token has no expiry.
		})
	}))
	t.Cleanup(fake.Close)

	vc := NewVerifyClient(fake.Client(), fake.URL)
	_, exp, err := vc.Verify(t.Context(), "argos_pat_aabbccdd_neverexpire")
	if err != nil {
		t.Fatalf("Verify() error = %v; want nil", err)
	}
	if !exp.IsZero() {
		t.Errorf("exp = %v; want zero time for token without expiry", exp)
	}
}

func TestVerifyClient_401_ReturnsErrVerifyDenied(t *testing.T) {
	t.Parallel()

	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"detail":"invalid token"}`))
	}))
	t.Cleanup(fake.Close)

	vc := NewVerifyClient(fake.Client(), fake.URL)
	_, _, err := vc.Verify(t.Context(), "argos_pat_aabbccdd_badtoken")
	if !errors.Is(err, ErrVerifyDenied) {
		t.Errorf("err = %v; want ErrVerifyDenied", err)
	}
}

func TestVerifyClient_500_ReturnsWrappedError(t *testing.T) {
	t.Parallel()

	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"detail":"internal error"}`))
	}))
	t.Cleanup(fake.Close)

	vc := NewVerifyClient(fake.Client(), fake.URL)
	_, _, err := vc.Verify(t.Context(), "argos_pat_aabbccdd_token")
	if err == nil {
		t.Fatal("expected error for 500 response; got nil")
	}
	if errors.Is(err, ErrVerifyDenied) {
		t.Error("5xx should NOT produce ErrVerifyDenied")
	}
}

func TestVerifyClient_RequestBodyShape(t *testing.T) {
	t.Parallel()

	const wantToken = "argos_pat_aabbccdd_exacttoken"
	var capturedBody []byte

	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capturedBody = body
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(fake.Close)

	vc := NewVerifyClient(fake.Client(), fake.URL)
	_, _, _ = vc.Verify(t.Context(), wantToken)

	var parsed struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(capturedBody, &parsed); err != nil {
		t.Fatalf("failed to unmarshal captured body: %v", err)
	}
	if parsed.Token != wantToken {
		t.Errorf("request body token = %q; want %q", parsed.Token, wantToken)
	}
	// The body must have exactly the `token` key — no extra fields.
	var raw map[string]any
	_ = json.Unmarshal(capturedBody, &raw)
	if len(raw) != 1 {
		t.Errorf("request body has %d keys; want exactly 1 (token)", len(raw))
	}
}

func TestVerifyClient_VMCollectorToken(t *testing.T) {
	t.Parallel()

	accountID := "a1b2c3d4-e5f6-7890-abcd-ef0123456789"
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(verifyResponseBody{
			Valid:               true,
			Kind:                "token",
			Scopes:              []string{"vm-collector"},
			BoundCloudAccountID: accountID,
		})
	}))
	t.Cleanup(fake.Close)

	vc := NewVerifyClient(fake.Client(), fake.URL)
	entry, _, err := vc.Verify(t.Context(), "argos_pat_aabbccdd_vmtoken")
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if entry.BoundCloudAccountID != accountID {
		t.Errorf("BoundCloudAccountID = %q; want %q", entry.BoundCloudAccountID, accountID)
	}
}
