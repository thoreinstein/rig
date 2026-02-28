package main

import (
	"context"
	"encoding/json"
	"log"
	"path/filepath"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	"thoreinstein.com/rig/pkg/plugin"
	"thoreinstein.com/rig/pkg/sdk"
)

type mockNodePlugin struct{}

func (p *mockNodePlugin) Info() sdk.Info {
	return sdk.Info{
		ID:         "test-node-plugin",
		APIVersion: plugin.APIVersion,
		SemVer:     "1.0.0",
		Capabilities: []sdk.Capability{
			{Name: plugin.NodeCapability, Version: "1.0"},
		},
	}
}

func (p *mockNodePlugin) ExecuteNode(ctx context.Context, req *apiv1.ExecuteNodeRequest) (*apiv1.ExecuteNodeResponse, error) {
	var config map[string]string
	if err := json.Unmarshal(req.ConfigJson, &config); err != nil {
		return &apiv1.ExecuteNodeResponse{ErrorMessage: "invalid config json"}, nil
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return &apiv1.ExecuteNodeResponse{ErrorMessage: "no metadata"}, nil
	}
	endpoints := md.Get("rig-resource-endpoint")
	if len(endpoints) == 0 {
		return &apiv1.ExecuteNodeResponse{ErrorMessage: "no rig-resource-endpoint metadata"}, nil
	}

	conn, err := grpc.NewClient("unix://"+endpoints[0], grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return &apiv1.ExecuteNodeResponse{ErrorMessage: err.Error()}, nil
	}
	defer conn.Close()
	resClient := apiv1.NewResourceServiceClient(conn)

	action := config["action"]
	switch action {
	case "read_allowed":
		path := filepath.Join(req.Workspace, "allowed.txt")
		_, err := resClient.ReadFile(ctx, &apiv1.ReadFileRequest{Path: path})
		if err != nil {
			return &apiv1.ExecuteNodeResponse{ErrorMessage: "read failed: " + err.Error()}, nil
		}
		return &apiv1.ExecuteNodeResponse{Output: []byte(`{"status":"success"}`)}, nil

	case "read_denied":
		path := "/etc/passwd"
		_, err := resClient.ReadFile(ctx, &apiv1.ReadFileRequest{Path: path})
		if err == nil {
			return &apiv1.ExecuteNodeResponse{ErrorMessage: "read should have been denied"}, nil
		}
		return &apiv1.ExecuteNodeResponse{Output: []byte(`{"status":"denied_as_expected"}`)}, nil

	case "network_allowed":
		_, err := resClient.HttpRequest(ctx, &apiv1.HttpRequestRequest{Url: "http://example.com", Method: "GET"})
		if err != nil {
			return &apiv1.ExecuteNodeResponse{ErrorMessage: "network request failed: " + err.Error()}, nil
		}
		return &apiv1.ExecuteNodeResponse{Output: []byte(`{"status":"success"}`)}, nil

	case "network_denied":
		_, err := resClient.HttpRequest(ctx, &apiv1.HttpRequestRequest{Url: "http://example.com", Method: "GET"})
		if err == nil {
			return &apiv1.ExecuteNodeResponse{ErrorMessage: "network request should have been denied"}, nil
		}
		return &apiv1.ExecuteNodeResponse{Output: []byte(`{"status":"denied_as_expected"}`)}, nil

	case "check_secrets":
		if req.Secrets["MY_API_KEY"] != "super-secret-value" {
			return &apiv1.ExecuteNodeResponse{ErrorMessage: "secret missing or incorrect"}, nil
		}
		return &apiv1.ExecuteNodeResponse{Output: []byte(`{"status":"success"}`)}, nil

	case "panic":
		panic("simulated plugin panic")

	default:
		return &apiv1.ExecuteNodeResponse{ErrorMessage: "unknown action"}, nil
	}
}

func main() {
	p := &mockNodePlugin{}
	if err := sdk.Serve(p); err != nil {
		log.Fatalf("failed to serve plugin: %v", err)
	}
}
