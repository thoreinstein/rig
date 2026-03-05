package plugin

import (
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

func TestGetSecret(t *testing.T) {
	proxy := NewHostSecretProxy()

	tests := []struct {
		name     string
		key      string
		envKey   string // if non-empty, set this env var before the call
		envVal   string
		wantVal  string
		wantCode codes.Code
		wantMsg  string
	}{
		{
			name:    "allowed key with value set returns value",
			key:     "JIRA_TOKEN",
			envKey:  "JIRA_TOKEN",
			envVal:  "tok-abc-123",
			wantVal: "tok-abc-123",
		},
		{
			name:     "allowed key not set in env returns PermissionDenied",
			key:      "BEADS_TOKEN",
			wantCode: codes.PermissionDenied,
			wantMsg:  "secret not available",
		},
		{
			name:     "disallowed key returns PermissionDenied",
			key:      "AWS_SECRET_ACCESS_KEY",
			wantCode: codes.PermissionDenied,
			wantMsg:  "secret not available",
		},
		{
			name:     "empty key returns PermissionDenied",
			key:      "",
			wantCode: codes.PermissionDenied,
			wantMsg:  "secret not available",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envKey != "" {
				t.Setenv(tc.envKey, tc.envVal)
			}

			resp, err := proxy.GetSecret(t.Context(), &apiv1.GetSecretRequest{Key: tc.key})

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

			if resp.Value != tc.wantVal {
				t.Errorf("value: got %q, want %q", resp.Value, tc.wantVal)
			}
		})
	}
}

func TestGetSecret_AllAllowListKeys(t *testing.T) {
	proxy := NewHostSecretProxy()

	for key := range secretAllowList {
		t.Run(key, func(t *testing.T) {
			want := "test-value-" + key
			t.Setenv(key, want)

			resp, err := proxy.GetSecret(t.Context(), &apiv1.GetSecretRequest{Key: key})
			if err != nil {
				t.Fatalf("unexpected error for allowed key %q: %v", key, err)
			}

			if resp.Value != want {
				t.Errorf("value: got %q, want %q", resp.Value, want)
			}
		})
	}
}

func TestGetSecret_AntiEnumeration(t *testing.T) {
	proxy := NewHostSecretProxy()

	// A disallowed key and an allowed-but-unset key must produce
	// identical gRPC error codes and messages so callers cannot
	// distinguish "key exists but not set" from "key not allowed."

	disallowedReq := &apiv1.GetSecretRequest{Key: "NOT_IN_ALLOW_LIST"}
	_, disallowedErr := proxy.GetSecret(t.Context(), disallowedReq)

	// BEADS_TOKEN is allowed but not set in this test's environment.
	unsetReq := &apiv1.GetSecretRequest{Key: "BEADS_TOKEN"}
	_, unsetErr := proxy.GetSecret(t.Context(), unsetReq)

	dSt, ok := status.FromError(disallowedErr)
	if !ok {
		t.Fatalf("disallowed error is not a gRPC status: %v", disallowedErr)
	}

	uSt, ok := status.FromError(unsetErr)
	if !ok {
		t.Fatalf("unset error is not a gRPC status: %v", unsetErr)
	}

	if dSt.Code() != uSt.Code() {
		t.Errorf("codes differ: disallowed=%v, unset=%v", dSt.Code(), uSt.Code())
	}

	if dSt.Message() != uSt.Message() {
		t.Errorf("messages differ: disallowed=%q, unset=%q", dSt.Message(), uSt.Message())
	}
}
