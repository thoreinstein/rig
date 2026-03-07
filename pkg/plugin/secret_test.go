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

	proxy := NewHostSecretProxy(resolver)
	jiraToken := "jira-tok-123"
	beadsToken := "beads-tok-456"
	proxy.RegisterPlugin(jiraToken, "jira")
	proxy.RegisterPlugin(beadsToken, "beads")

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

func TestGetSecret_InternalError(t *testing.T) {
	// A resolver that returns an error NOT wrapping ErrSecretNotFound
	// should produce codes.Internal, not codes.NotFound.
	resolver := func(_, _ string) (string, error) {
		return "", rigerrors.New("keychain access failed")
	}

	proxy := NewHostSecretProxy(resolver)
	proxy.RegisterPlugin("tok", "myplugin")

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
	proxy := NewHostSecretProxy(resolver)
	token := "tok"
	proxy.RegisterPlugin(token, "p")

	_, err := proxy.GetSecret(t.Context(), &apiv1.GetSecretRequest{Key: "k", Token: token})
	if err != nil {
		t.Fatalf("expected success before unregister, got %v", err)
	}

	proxy.UnregisterPlugin(token)

	_, err = proxy.GetSecret(t.Context(), &apiv1.GetSecretRequest{Key: "k", Token: token})
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated after unregister, got %v", err)
	}
}
