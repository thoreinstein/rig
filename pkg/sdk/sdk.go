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
