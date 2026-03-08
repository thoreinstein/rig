package plugin

import (
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	rigerrors "thoreinstein.com/rig/pkg/errors"
)

func TestGetSecret(t *testing.T) {
	// Mock secrets indexed by pluginName -> secretKey -> value.
	mockSecrets := map[string]map[string]string{
		"jira":  {"token": "jira-secret"},
		"beads": {"api": "beads-secret"},
	}

	resolver := func(pluginName, secretKey string) (string, error) {
		secrets, ok := mockSecrets[pluginName]
		if !ok {
			return "", rigerrors.Wrap(ErrSecretNotFound, fmt.Sprintf("plugin %q not found", pluginName))
		}
		val, ok := secrets[secretKey]
		if !ok {
			return "", rigerrors.Wrap(ErrSecretNotFound, fmt.Sprintf("secret %q not found", secretKey))
		}
		return val, nil
	}

	store := newTokenStore()
	proxy := NewHostSecretProxy(store, resolver)
	jiraToken := "jira-tok-123"
	beadsToken := "beads-tok-456"
	store.Register(jiraToken, "jira")
	store.Register(beadsToken, "beads")

	tests := []struct {
		name     string
		key      string
		token    string
		wantVal  string
		wantCode codes.Code
		wantMsg  string
	}{
		{
			name:    "valid token and allowed key returns value",
			key:     "token",
			token:   jiraToken,
			wantVal: "jira-secret",
		},
		{
			name:    "valid token for another plugin returns its own value",
			key:     "api",
			token:   beadsToken,
			wantVal: "beads-secret",
		},
		{
			name:     "invalid token returns Unauthenticated",
			key:      "token",
			token:    "wrong-token",
			wantCode: codes.Unauthenticated,
			wantMsg:  "invalid secret token",
		},
		{
			name:     "valid token but missing key returns NotFound",
			key:      "wrong-key",
			token:    jiraToken,
			wantCode: codes.NotFound,
			wantMsg:  "secret not available",
		},
		{
			name:     "plugin cannot access another plugin's secret",
			key:      "api", // jira doesn't have 'api' secret, only beads does
			token:    jiraToken,
			wantCode: codes.NotFound,
			wantMsg:  "secret not available",
		},
		{
			name:     "dotted key is rejected",
			key:      "beads.secrets.api",
			token:    jiraToken,
			wantCode: codes.InvalidArgument,
			wantMsg:  "invalid secret key",
		},
		{
			name:     "empty key is rejected",
			key:      "",
			token:    jiraToken,
			wantCode: codes.InvalidArgument,
			wantMsg:  "invalid secret key",
		},
		{
			name:     "null byte in key is rejected",
			key:      "token\x00other",
			token:    jiraToken,
			wantCode: codes.InvalidArgument,
			wantMsg:  "invalid secret key",
		},
		{
			name:     "path separator in key is rejected",
			key:      "../etc/passwd",
			token:    jiraToken,
			wantCode: codes.InvalidArgument,
			wantMsg:  "invalid secret key",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := proxy.GetSecret(t.Context(), &apiv1.GetSecretRequest{
				Key:   tc.key,
				Token: tc.token,
			})

			if tc.wantCode != codes.OK {
				if err == nil {
					t.Fatalf("expected error with code %v, got nil", tc.wantCode)
				}

				st, ok := status.FromError(err)
				if !ok {
					t.Fatalf("expected gRPC status error, got %T: %v", err, err)
				}

				if st.Code() != tc.wantCode {
					t.Errorf("code: got %v, want %v", st.Code(), tc.wantCode)
				}

				if st.Message() != tc.wantMsg {
					t.Errorf("message: got %q, want %q", st.Message(), tc.wantMsg)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.Secret == nil {
				t.Fatal("secret is nil in response")
			}

			if resp.Secret.Value != tc.wantVal {
				t.Errorf("value: got %q, want %q", resp.Secret.Value, tc.wantVal)
			}
		})
	}
}

func TestGetSecrets(t *testing.T) {
	mockSecrets := map[string]map[string]string{
		"jira": {"token": "jira-secret", "url": "https://jira.example.com"},
	}

	resolver := func(pluginName, secretKey string) (string, error) {
		secrets, ok := mockSecrets[pluginName]
		if !ok {
			return "", rigerrors.Wrap(ErrSecretNotFound, fmt.Sprintf("plugin %q not found", pluginName))
		}
		val, ok := secrets[secretKey]
		if !ok {
			return "", rigerrors.Wrap(ErrSecretNotFound, fmt.Sprintf("secret %q not found", secretKey))
		}
		return val, nil
	}

	store := newTokenStore()
	proxy := NewHostSecretProxy(store, resolver)
	jiraToken := "jira-tok-123"
	store.Register(jiraToken, "jira")

	tests := []struct {
		name     string
		keys     []string
		token    string
		wantKeys []string
		wantCode codes.Code
	}{
		{
			name:     "all keys found",
			keys:     []string{"token", "url"},
			token:    jiraToken,
			wantKeys: []string{"token", "url"},
		},
		{
			name:     "partial keys found",
			keys:     []string{"token", "missing"},
			token:    jiraToken,
			wantKeys: []string{"token"},
		},
		{
			name:     "no keys found",
			keys:     []string{"missing1", "missing2"},
			token:    jiraToken,
			wantKeys: []string{},
		},
		{
			name:     "invalid keys are omitted",
			keys:     []string{"token", "bad.key", "../path", "url"},
			token:    jiraToken,
			wantKeys: []string{"token", "url"},
		},
		{
			name:     "empty keys list",
			keys:     []string{},
			token:    jiraToken,
			wantKeys: []string{},
		},
		{
			name:     "invalid token",
			keys:     []string{"token"},
			token:    "wrong-token",
			wantCode: codes.Unauthenticated,
		},
		{
			name:     "too many keys returns InvalidArgument",
			keys:     make([]string, maxBulkSecretKeys+1),
			token:    jiraToken,
			wantCode: codes.InvalidArgument,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := proxy.GetSecrets(t.Context(), &apiv1.GetSecretsRequest{
				Keys:  tc.keys,
				Token: tc.token,
			})

			if tc.wantCode != codes.OK {
				if err == nil {
					t.Fatalf("expected error with code %v, got nil", tc.wantCode)
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Fatalf("expected gRPC status error, got %T: %v", err, err)
				}
				if st.Code() != tc.wantCode {
					t.Errorf("code: got %v, want %v", st.Code(), tc.wantCode)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(resp.Secrets) != len(tc.wantKeys) {
				t.Fatalf("secrets count: got %d, want %d", len(resp.Secrets), len(tc.wantKeys))
			}

			for _, key := range tc.wantKeys {
				sv, ok := resp.Secrets[key]
				if !ok {
					t.Errorf("expected key %q in response", key)
					continue
				}
				if sv.Value != mockSecrets["jira"][key] {
					t.Errorf("key %q: got %q, want %q", key, sv.Value, mockSecrets["jira"][key])
				}
			}
		})
	}
}

func TestRefreshToken(t *testing.T) {
	resolver := func(_, _ string) (string, error) { return "val", nil }

	tests := []struct {
		name       string
		token      string
		registered bool
		wantCode   codes.Code
	}{
		{
			name:       "successful rotation returns new token",
			token:      "original-tok",
			registered: true,
		},
		{
			name:     "invalid token returns Unauthenticated",
			token:    "nonexistent",
			wantCode: codes.Unauthenticated,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newTokenStore()
			proxy := NewHostSecretProxy(store, resolver)

			if tc.registered {
				store.Register(tc.token, "myplugin")
			}

			resp, err := proxy.RefreshToken(t.Context(), &apiv1.RefreshTokenRequest{
				CurrentToken: tc.token,
			})

			if tc.wantCode != codes.OK {
				if err == nil {
					t.Fatalf("expected error with code %v, got nil", tc.wantCode)
				}
				if status.Code(err) != tc.wantCode {
					t.Errorf("code: got %v, want %v", status.Code(err), tc.wantCode)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.NewToken == "" {
				t.Fatal("new token is empty")
			}
			if resp.NewToken == tc.token {
				t.Error("new token should differ from original")
			}

			// Old token should no longer resolve.
			_, getErr := proxy.GetSecret(t.Context(), &apiv1.GetSecretRequest{Key: "k", Token: tc.token})
			if status.Code(getErr) != codes.Unauthenticated {
				t.Errorf("old token: expected Unauthenticated, got %v", getErr)
			}

			// New token should resolve.
			_, getErr = proxy.GetSecret(t.Context(), &apiv1.GetSecretRequest{Key: "k", Token: resp.NewToken})
			if getErr != nil {
				t.Errorf("new token: unexpected error: %v", getErr)
			}
		})
	}
}

func TestTokenStore_UnregisterPlugin(t *testing.T) {
	store := newTokenStore()
	name := "my-plugin"
	t1 := "token-1"
	t2 := "token-2"

	store.Register(t1, name)
	store.Register(t2, name)
	store.Register("other-token", "other-plugin")

	store.UnregisterPlugin(name)

	if _, ok := store.Resolve(t1); ok {
		t.Errorf("token %s still exists after UnregisterPlugin", t1)
	}
	if _, ok := store.Resolve(t2); ok {
		t.Errorf("token %s still exists after UnregisterPlugin", t2)
	}
	if _, ok := store.Resolve("other-token"); !ok {
		t.Error("other-token was incorrectly removed")
	}
}

func TestGetSecret_InternalError(t *testing.T) {
	// A resolver that returns an error NOT wrapping ErrSecretNotFound
	// should produce codes.Internal, not codes.NotFound.
	resolver := func(_, _ string) (string, error) {
		return "", rigerrors.New("keychain access failed")
	}

	store := newTokenStore()
	proxy := NewHostSecretProxy(store, resolver)
	store.Register("tok", "myplugin")

	_, err := proxy.GetSecret(t.Context(), &apiv1.GetSecretRequest{Key: "k", Token: "tok"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T: %v", err, err)
	}
	if st.Code() != codes.Internal {
		t.Errorf("code: got %v, want %v", st.Code(), codes.Internal)
	}
	if st.Message() != "secret resolution failed" {
		t.Errorf("message: got %q, want %q", st.Message(), "secret resolution failed")
	}
}

func TestGetSecret_Unregister(t *testing.T) {
	resolver := func(_, _ string) (string, error) { return "val", nil }
	store := newTokenStore()
	proxy := NewHostSecretProxy(store, resolver)
	token := "tok"
	store.Register(token, "p")

	_, err := proxy.GetSecret(t.Context(), &apiv1.GetSecretRequest{Key: "k", Token: token})
	if err != nil {
		t.Fatalf("expected success before unregister, got %v", err)
	}

	store.Unregister(token)

	_, err = proxy.GetSecret(t.Context(), &apiv1.GetSecretRequest{Key: "k", Token: token})
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated after unregister, got %v", err)
	}
}
