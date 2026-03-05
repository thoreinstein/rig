package main

import (
	"fmt"
	"log"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	"thoreinstein.com/rig/pkg/sdk"
)

type plugin struct{}

func (p *plugin) Info() sdk.Info {
	return sdk.Info{
		ID:         "secret-cmd",
		APIVersion: "v1",
		SemVer:     "0.1.0",
		Capabilities: []sdk.Capability{
			{Name: "command", Version: "1.0.0"},
		},
		Commands: []sdk.CommandDescriptor{
			{
				Name:  "get-secret",
				Short: "Fetch a host secret",
				Long:  "Resolves api_key from the Rig host secret service",
			},
		},
	}
}

func (p *plugin) Execute(req *apiv1.ExecuteRequest, stream apiv1.CommandService_ExecuteServer) error {
	if req.Command != "get-secret" {
		if err := stream.Send(&apiv1.ExecuteResponse{
			Stderr: []byte("Unknown command: " + req.Command),
		}); err != nil {
			return err
		}
		return stream.Send(&apiv1.ExecuteResponse{
			ExitCode: 1,
			Done:     true,
		})
	}

	secret, err := sdk.GetSecret(stream.Context(), "api_key")
	if err != nil {
		if sendErr := stream.Send(&apiv1.ExecuteResponse{
			Stderr: []byte(fmt.Sprintf("secret lookup failed: %v", err)),
		}); sendErr != nil {
			return sendErr
		}
		return stream.Send(&apiv1.ExecuteResponse{
			ExitCode: 1,
			Done:     true,
		})
	}

	if err := stream.Send(&apiv1.ExecuteResponse{
		Stdout: []byte(secret),
	}); err != nil {
		return err
	}
	return stream.Send(&apiv1.ExecuteResponse{
		ExitCode: 0,
		Done:     true,
	})
}

func main() {
	if err := sdk.Serve(&plugin{}); err != nil {
		log.Fatal(err)
	}
}
