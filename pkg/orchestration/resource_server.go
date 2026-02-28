package orchestration

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

// resourceServer implements the ResourceService to proxy requests from a plugin,
// enforcing the capabilities granted to the node.
type resourceServer struct {
	apiv1.UnimplementedResourceServiceServer
	nodeID string
	caps   *NodeCapabilities
	client *http.Client
}

// newResourceServer creates a new ResourceService server for a specific node execution.
func newResourceServer(nodeID string, caps *NodeCapabilities) *resourceServer {
	return &resourceServer{
		nodeID: nodeID,
		caps:   caps,
		client: &http.Client{},
	}
}

// checkPath verifies if the path is permitted according to the node's capabilities.
func (s *resourceServer) checkPath(requested string) (string, error) {
	if s.caps == nil {
		return "", status.Errorf(codes.PermissionDenied, "node %s has no capabilities", s.nodeID)
	}
	if !filepath.IsAbs(requested) {
		return "", status.Errorf(codes.InvalidArgument, "path must be absolute")
	}
	cleaned := filepath.Clean(requested)
	if !s.caps.IsPathAllowed(cleaned) {
		return "", status.Errorf(codes.PermissionDenied, "access denied for node %s", s.nodeID)
	}
	return cleaned, nil
}

func (s *resourceServer) ReadFile(ctx context.Context, req *apiv1.ReadFileRequest) (*apiv1.ReadFileResponse, error) {
	path, err := s.checkPath(req.Path)
	if err != nil {
		return nil, err
	}
	// Cap file reads to 10MB to prevent OOM on very large files.
	const maxFileSize = 10 << 20
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, "file not found: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to open file: %v", err)
	}
	defer f.Close()
	content, err := io.ReadAll(io.LimitReader(f, maxFileSize))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read file: %v", err)
	}
	return &apiv1.ReadFileResponse{Content: content}, nil
}

func (s *resourceServer) WriteFile(ctx context.Context, req *apiv1.WriteFileRequest) (*apiv1.WriteFileResponse, error) {
	path, err := s.checkPath(req.Path)
	if err != nil {
		return nil, err
	}
	// Use 0644 as fallback if 0 is provided.
	mode := os.FileMode(req.Mode)
	if mode == 0 {
		mode = 0o644
	}
	if err := os.WriteFile(path, req.Content, mode); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to write file: %v", err)
	}
	return &apiv1.WriteFileResponse{}, nil
}

func (s *resourceServer) ListDir(ctx context.Context, req *apiv1.ListDirRequest) (*apiv1.ListDirResponse, error) {
	path, err := s.checkPath(req.Path)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, "directory not found: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to list directory: %v", err)
	}

	infos := make([]*apiv1.ListDirResponse_FileInfo, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue // skip entries that disappear during read
		}
		infos = append(infos, &apiv1.ListDirResponse_FileInfo{
			Name:  e.Name(),
			IsDir: e.IsDir(),
			Size:  info.Size(),
		})
	}
	return &apiv1.ListDirResponse{Entries: infos}, nil
}

func (s *resourceServer) HttpRequest(ctx context.Context, req *apiv1.HttpRequestRequest) (*apiv1.HttpRequestResponse, error) {
	if s.caps == nil || !s.caps.NetworkAccess {
		return nil, status.Errorf(codes.PermissionDenied, "network access denied for node %s", s.nodeID)
	}

	var bodyReader io.Reader
	if len(req.Body) > 0 {
		bodyReader = strings.NewReader(string(req.Body))
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.Url, bodyReader)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid http request: %v", err)
	}

	for k, v := range req.Headers {
		httpReq.Header.Add(k, v)
	}

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "http request failed: %v", err)
	}
	defer resp.Body.Close()

	// Cap response body to 10MB to prevent OOM from a misbehaving upstream.
	const maxResponseSize = 10 << 20
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read response body: %v", err)
	}

	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			respHeaders[k] = v[0]
		}
	}

	return &apiv1.HttpRequestResponse{
		StatusCode: int32(resp.StatusCode),
		Headers:    respHeaders,
		Body:       respBody,
	}, nil
}
