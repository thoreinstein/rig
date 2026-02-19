package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"google.golang.org/grpc"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

type pluginService struct {
	apiv1.UnimplementedPluginServiceServer
}

func (s *pluginService) Handshake(ctx context.Context, req *apiv1.HandshakeRequest) (*apiv1.HandshakeResponse, error) {
	return &apiv1.HandshakeResponse{
		PluginId:     "mock-cmd",
		ApiVersion:   "v1",
		PluginSemver: "0.1.0",
		Capabilities: []*apiv1.Capability{
			{Name: "command", Version: "1.0.0"},
		},
		Commands: []*apiv1.CommandDescriptorProto{
			{
				Name:  "echo",
				Short: "Echo arguments",
				Long:  "Echoes all provided arguments back to stdout",
			},
		},
	}, nil
}

type commandService struct {
	apiv1.UnimplementedCommandServiceServer
}

func (s *commandService) Execute(req *apiv1.ExecuteRequest, stream apiv1.CommandService_ExecuteServer) error {
	if req.Command == "echo" {
		output := strings.Join(req.Args, " ")
		err := stream.Send(&apiv1.ExecuteResponse{
			Stdout: []byte(output),
		})
		if err != nil {
			return err
		}
	} else {
		err := stream.Send(&apiv1.ExecuteResponse{
			Stderr: []byte(fmt.Sprintf("Unknown command: %s", req.Command)),
		})
		if err != nil {
			return err
		}
		return stream.Send(&apiv1.ExecuteResponse{
			ExitCode: 1,
			Done:     true,
		})
	}

	return stream.Send(&apiv1.ExecuteResponse{
		ExitCode: 0,
		Done:     true,
	})
}

func main() {
	endpoint := os.Getenv("RIG_PLUGIN_ENDPOINT")
	if endpoint == "" {
		fmt.Fprintln(os.Stderr, "RIG_PLUGIN_ENDPOINT not set")
		os.Exit(1)
	}

	// Remove socket if it exists (though host usually handles this)
	_ = os.Remove(endpoint)

	lis, err := net.Listen("unix", endpoint)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to listen: %v\n", err)
		os.Exit(1)
	}

	srv := grpc.NewServer()
	apiv1.RegisterPluginServiceServer(srv, &pluginService{})
	apiv1.RegisterCommandServiceServer(srv, &commandService{})

	if err := srv.Serve(lis); err != nil {
		fmt.Fprintf(os.Stderr, "failed to serve: %v\n", err)
		os.Exit(1)
	}
}
