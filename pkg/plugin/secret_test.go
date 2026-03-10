package plugin

import (
	"context"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	rigerrors "thoreinstein.com/rig/pkg/errors"
)

func withPlugin(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, pluginNameKey, name)
}

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

	proxy := NewHostSecretProxy(resolver, nil)

	tests := []struct {
		name       string
		key        string
		pluginName string
		wantVal    string
		wantCode   codes.Code
		wantMsg    string
	}{
		{
			name:       "valid plugin and allowed key returns value",
			key:        "token",
			pluginName: "jira",
			wantVal:    "jira-secret",
		},
		{
			name:       "another valid plugin returns its own value",
			key:        "api",
			pluginName: "beads",
			wantVal:    "beads-secret",
		},
		{
			name:     "missing plugin identity returns Unauthenticated",
			key:      "token",
			wantCode: codes.Unauthenticated,
			wantMsg:  "missing plugin identity",
		},
		{
			name:       "valid plugin but missing key returns NotFound",
			key:        "wrong-key",
			pluginName: "jira",
			wantCode:   codes.NotFound,
			wantMsg:    "secret not available",
		},
		{
			name:       "plugin cannot access another plugin's secret",
			key:        "api", // jira doesn't have 'api' secret, only beads does
			pluginName: "jira",
			wantCode:   codes.NotFound,
			wantMsg:    "secret not available",
		},
		{
			name:       "dotted key is rejected",
			key:        "beads.secrets.api",
			pluginName: "jira",
			wantCode:   codes.InvalidArgument,
			wantMsg:    "invalid secret key",
		},
		{
			name:       "empty key is rejected",
			key:        "",
			pluginName: "jira",
			wantCode:   codes.InvalidArgument,
			wantMsg:    "invalid secret key",
		},
		{
			name:       "null byte in key is rejected",
			key:        "token\x00other",
			pluginName: "jira",
			wantCode:   codes.InvalidArgument,
			wantMsg:    "invalid secret key",
		},
		{
			name:       "path separator in key is rejected",
			key:        "../etc/passwd",
			pluginName: "jira",
			wantCode:   codes.InvalidArgument,
			wantMsg:    "invalid secret key",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			if tc.pluginName != "" {
				ctx = withPlugin(ctx, tc.pluginName)
			}

			resp, err := proxy.GetSecret(ctx, &apiv1.GetSecretRequest{
				Key: tc.key,
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

	proxy := NewHostSecretProxy(resolver, nil)

	tests := []struct {
		name       string
		keys       []string
		pluginName string
		wantKeys   []string
		wantCode   codes.Code
	}{
		{
			name:       "all keys found",
			keys:       []string{"token", "url"},
			pluginName: "jira",
			wantKeys:   []string{"token", "url"},
		},
		{
			name:       "partial keys found",
			keys:       []string{"token", "missing"},
			pluginName: "jira",
			wantKeys:   []string{"token"},
		},
		{
			name:       "no keys found",
			keys:       []string{"missing1", "missing2"},
			pluginName: "jira",
			wantKeys:   []string{},
		},
		{
			name:       "invalid keys are omitted",
			keys:       []string{"token", "bad.key", "../path", "url"},
			pluginName: "jira",
			wantKeys:   []string{"token", "url"},
		},
		{
			name:       "empty keys list",
			keys:       []string{},
			pluginName: "jira",
			wantKeys:   []string{},
		},
		{
			name:     "missing plugin identity",
			keys:     []string{"token"},
			wantCode: codes.Unauthenticated,
		},
		{
			name:       "too many keys returns InvalidArgument",
			keys:       make([]string, maxBulkSecretKeys+1),
			pluginName: "jira",
			wantCode:   codes.InvalidArgument,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			if tc.pluginName != "" {
				ctx = withPlugin(ctx, tc.pluginName)
			}

			resp, err := proxy.GetSecrets(ctx, &apiv1.GetSecretsRequest{
				Keys: tc.keys,
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

func TestRefreshToken_DeprecatedNoOp(t *testing.T) {
	resolver := func(_, _ string) (string, error) { return "val", nil }
	proxy := NewHostSecretProxy(resolver, nil)

	resp, err := proxy.RefreshToken(t.Context(), &apiv1.RefreshTokenRequest{
		CurrentToken: "some-token",
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestGetSecret_InternalError(t *testing.T) {
	// A resolver that returns an error NOT wrapping ErrSecretNotFound
	// should produce codes.Internal, not codes.NotFound.
	resolver := func(_, _ string) (string, error) {
		return "", rigerrors.New("keychain access failed")
	}

	proxy := NewHostSecretProxy(resolver, nil)
	ctx := withPlugin(t.Context(), "myplugin")

	_, err := proxy.GetSecret(ctx, &apiv1.GetSecretRequest{Key: "k"})
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
