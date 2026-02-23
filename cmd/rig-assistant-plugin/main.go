package main

import (
	"context"
	"log"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	"thoreinstein.com/rig/pkg/sdk"
)

type assistant struct{}

func (a *assistant) Info() sdk.Info {
	return sdk.Info{
		ID:         "sample-assistant",
		APIVersion: "v1",
		SemVer:     "v0.1.0",
		Capabilities: []sdk.Capability{
			{Name: "assistant", Version: "1.0.0"},
		},
	}
}

func (a *assistant) Chat(ctx context.Context, req *apiv1.ChatRequest) (*apiv1.ChatResponse, error) {
	return &apiv1.ChatResponse{
		Content:      "This is a sample AI response.",
		StopReason:   "end_turn",
		InputTokens:  10,
		OutputTokens: 20,
	}, nil
}

func (a *assistant) StreamChat(req *apiv1.StreamChatRequest, stream apiv1.AssistantService_StreamChatServer) error {
	words := []string{"Hello,", " I", " am", " a", " sample", " AI", " plugin!"}
	for i, word := range words {
		if err := stream.Send(&apiv1.StreamChatResponse{
			Content: word,
			Done:    i == len(words)-1,
		}); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if err := sdk.Serve(&assistant{}); err != nil {
		log.Fatal(err)
	}
}
