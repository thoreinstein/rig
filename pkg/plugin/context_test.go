package plugin

import (
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

func TestHostContextProxy_GetContext(t *testing.T) {
	store := newTokenStore()
	ctx := PluginContext{
		ProjectRoot:  "/project",
		WorktreeRoot: "/worktree",
		TicketID:     "RIG-123",
		Metadata:     map[string]any{"key": "value"},
	}
	proxy := NewHostContextProxy(store, ctx)

	token := "valid-token"
	store.Register(token, "test-plugin")

	t.Run("valid token returns context", func(t *testing.T) {
		resp, err := proxy.GetContext(t.Context(), &apiv1.GetContextRequest{
			Token: token,
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.ProjectRoot != ctx.ProjectRoot {
			t.Errorf("ProjectRoot: got %q, want %q", resp.ProjectRoot, ctx.ProjectRoot)
		}
		if resp.WorktreeRoot != ctx.WorktreeRoot {
			t.Errorf("WorktreeRoot: got %q, want %q", resp.WorktreeRoot, ctx.WorktreeRoot)
		}
		if resp.TicketId != ctx.TicketID {
			t.Errorf("TicketId: got %q, want %q", resp.TicketId, ctx.TicketID)
		}
		if resp.Metadata.AsMap()["key"] != "value" {
			t.Errorf("Metadata key: got %v, want %v", resp.Metadata.AsMap()["key"], "value")
		}
	})

	t.Run("invalid token returns Unauthenticated", func(t *testing.T) {
		_, err := proxy.GetContext(t.Context(), &apiv1.GetContextRequest{
			Token: "invalid-token",
		})

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})
}
