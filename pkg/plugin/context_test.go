package plugin

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

func TestHostContextProxy_GetContext(t *testing.T) {
	pCtx := PluginContext{
		ProjectRoot:  "/project",
		WorktreeRoot: "/worktree",
		TicketID:     "RIG-123",
		Metadata:     map[string]any{"key": "value"},
	}
	proxy := NewHostContextProxy(pCtx, nil)

	tests := []struct {
		name             string
		pluginName       string
		wantCode         codes.Code
		wantProjectRoot  string
		wantWorktreeRoot string
		wantTicketID     string
		wantMetadataKey  string
		wantMetadataVal  any
	}{
		{
			name:             "valid identity returns context",
			pluginName:       "test-plugin",
			wantProjectRoot:  "/project",
			wantWorktreeRoot: "/worktree",
			wantTicketID:     "RIG-123",
			wantMetadataKey:  "key",
			wantMetadataVal:  "value",
		},
		{
			name:       "missing identity returns Unauthenticated",
			pluginName: "",
			wantCode:   codes.Unauthenticated,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			if tc.pluginName != "" {
				ctx = context.WithValue(ctx, pluginNameKey, tc.pluginName)
			}

			resp, err := proxy.GetContext(ctx, &apiv1.GetContextRequest{})

			if tc.wantCode != codes.OK {
				if err == nil {
					t.Fatalf("expected error with code %v, got nil", tc.wantCode)
				}
				st, ok := status.FromError(err)
				if !ok || st.Code() != tc.wantCode {
					t.Errorf("code: got %v, want %v", status.Code(err), tc.wantCode)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.ProjectRoot != tc.wantProjectRoot {
				t.Errorf("ProjectRoot: got %q, want %q", resp.ProjectRoot, tc.wantProjectRoot)
			}
			if resp.WorktreeRoot != tc.wantWorktreeRoot {
				t.Errorf("WorktreeRoot: got %q, want %q", resp.WorktreeRoot, tc.wantWorktreeRoot)
			}
			if resp.TicketId != tc.wantTicketID {
				t.Errorf("TicketId: got %q, want %q", resp.TicketId, tc.wantTicketID)
			}
			if tc.wantMetadataKey != "" {
				if resp.Metadata.AsMap()[tc.wantMetadataKey] != tc.wantMetadataVal {
					t.Errorf("Metadata[%s]: got %v, want %v", tc.wantMetadataKey, resp.Metadata.AsMap()[tc.wantMetadataKey], tc.wantMetadataVal)
				}
			}
		})
	}
}
