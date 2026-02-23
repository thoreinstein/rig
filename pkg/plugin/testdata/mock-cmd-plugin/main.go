package main

import (
	"log"
	"strings"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	"thoreinstein.com/rig/pkg/sdk"
)

type plugin struct{}

func (p *plugin) Info() sdk.Info {
	return sdk.Info{
		ID:         "mock-cmd",
		APIVersion: "v1",
		SemVer:     "0.1.0",
		Capabilities: []sdk.Capability{
			{Name: "command", Version: "1.0.0"},
		},
		Commands: []sdk.CommandDescriptor{
			{
				Name:  "echo",
				Short: "Echo arguments",
				Long:  "Echoes all provided arguments back to stdout",
			},
		},
	}
}

func (p *plugin) Execute(req *apiv1.ExecuteRequest, stream apiv1.CommandService_ExecuteServer) error {
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
			Stderr: []byte("Unknown command: " + req.Command),
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
	if err := sdk.Serve(&plugin{}); err != nil {
		log.Fatal(err)
	}
}
