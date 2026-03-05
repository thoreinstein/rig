package sdk

import (
	"context"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

// PluginInfo is the mandatory interface for all Rig plugins.
// It provides the metadata used during the Handshake process.
type PluginInfo interface {
	Info() Info
}

// Info represents the plugin's metadata.
type Info struct {
	// ID is the unique identifier for the plugin.
	ID string
	// APIVersion is the version of the Plugin API implemented by the plugin.
	APIVersion string
	// SemVer is the semantic version of the plugin binary itself.
	SemVer string
	// Capabilities is a list of features the plugin supports.
	Capabilities []Capability
	// Commands is a list of CLI commands provided by the plugin.
	Commands []CommandDescriptor
}

// Capability represents a specific feature or integration point provided by a plugin.
type Capability struct {
	Name    string
	Version string
}

// CommandDescriptor represents a CLI command provided by a plugin.
type CommandDescriptor struct {
	Name    string
	Short   string
	Long    string
	Aliases []string
}

// AssistantHandler is an optional interface for plugins providing AI assistant capabilities.
type AssistantHandler interface {
	Chat(ctx context.Context, req *apiv1.ChatRequest) (*apiv1.ChatResponse, error)
	StreamChat(req *apiv1.StreamChatRequest, server apiv1.AssistantService_StreamChatServer) error
}

// CommandHandler is an optional interface for plugins providing custom CLI commands.
type CommandHandler interface {
	Execute(req *apiv1.ExecuteRequest, server apiv1.CommandService_ExecuteServer) error
}

// NodeHandler is an optional interface for plugins providing workflow node execution capabilities.
type NodeHandler interface {
	ExecuteNode(ctx context.Context, req *apiv1.ExecuteNodeRequest) (*apiv1.ExecuteNodeResponse, error)
}

// VCSHandler is an optional interface for plugins providing Version Control System capabilities.
type VCSHandler interface {
	GetRepoRoot(ctx context.Context, req *apiv1.GetRepoRootRequest) (*apiv1.GetRepoRootResponse, error)
	GetRepoName(ctx context.Context, req *apiv1.GetRepoNameRequest) (*apiv1.GetRepoNameResponse, error)
	GetDefaultBranch(ctx context.Context, req *apiv1.GetDefaultBranchRequest) (*apiv1.GetDefaultBranchResponse, error)
	CreateWorktree(ctx context.Context, req *apiv1.CreateWorktreeRequest) (*apiv1.CreateWorktreeResponse, error)
	ListWorktrees(ctx context.Context, req *apiv1.ListWorktreesRequest) (*apiv1.ListWorktreesResponse, error)
	RemoveWorktree(ctx context.Context, req *apiv1.RemoveWorktreeRequest) (*apiv1.RemoveWorktreeResponse, error)
	ForceRemoveWorktree(ctx context.Context, req *apiv1.ForceRemoveWorktreeRequest) (*apiv1.ForceRemoveWorktreeResponse, error)
	GetWorktreePath(ctx context.Context, req *apiv1.GetWorktreePathRequest) (*apiv1.GetWorktreePathResponse, error)
	Clone(ctx context.Context, req *apiv1.CloneRequest) (*apiv1.CloneResponse, error)
	IsBranchMerged(ctx context.Context, req *apiv1.IsBranchMergedRequest) (*apiv1.IsBranchMergedResponse, error)
}

// TicketHandler is an optional interface for plugins providing ticketing integration.
type TicketHandler interface {
	GetTicketInfo(ctx context.Context, req *apiv1.GetTicketInfoRequest) (*apiv1.GetTicketInfoResponse, error)
	UpdateTicketStatus(ctx context.Context, req *apiv1.UpdateTicketStatusRequest) (*apiv1.UpdateTicketStatusResponse, error)
	ListTransitions(ctx context.Context, req *apiv1.ListTransitionsRequest) (*apiv1.ListTransitionsResponse, error)
}

// Configurable is an optional interface for plugins that need to receive configuration
// from the host during the Handshake process.
type Configurable interface {
	Configure(configJSON []byte) error
}
