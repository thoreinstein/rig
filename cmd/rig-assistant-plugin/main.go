package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

type server struct {
	apiv1.UnimplementedPluginServiceServer
	apiv1.UnimplementedAssistantServiceServer
}

func (s *server) Handshake(ctx context.Context, req *apiv1.HandshakeRequest) (*apiv1.HandshakeResponse, error) {
	return &apiv1.HandshakeResponse{
		PluginId:      "sample-assistant",
		ApiVersion:    "v1",
		PluginSemver:  "v0.1.0",
		Capabilities: []*apiv1.Capability{
			{Name: "assistant", Version: "1.0.0"},
		},
	}, nil
}

func (s *server) Chat(ctx context.Context, req *apiv1.ChatRequest) (*apiv1.ChatResponse, error) {
	return &apiv1.ChatResponse{
		Content:      "This is a sample AI response.",
		StopReason:   "end_turn",
		InputTokens:  10,
		OutputTokens: 20,
	}, nil
}

func (s *server) StreamChat(req *apiv1.ChatRequest, stream apiv1.AssistantService_StreamChatServer) error {
	words := []string{"Hello,", " I", " am", " a", " sample", " AI", " plugin!"}
	for i, word := range words {
		if err := stream.Send(&apiv1.ChatChunk{
			Content: word,
			Done:    i == len(words)-1,
		}); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	endpoint := os.Getenv("RIG_PLUGIN_ENDPOINT")
	if endpoint == "" {
		log.Fatal("RIG_PLUGIN_ENDPOINT environment variable not set")
	}

	// Remove socket if it already exists
	_ = os.Remove(endpoint)

	lis, err := net.Listen("unix", endpoint)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	apiv1.RegisterPluginServiceServer(s, &server{})
	apiv1.RegisterAssistantServiceServer(s, &server{})

	// Handle graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		s.GracefulStop()
	}()

	fmt.Printf("Plugin starting on %s\n", endpoint)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
