package daemon

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	"thoreinstein.com/rig/pkg/plugin"
)

// DaemonServer implements apiv1.DaemonServiceServer.
type DaemonServer struct {
	apiv1.UnimplementedDaemonServiceServer
	manager    *plugin.Manager
	uiProxy    *DaemonUIProxy
	logger     *slog.Logger
	startTime  time.Time
	rigVersion string

	mu               sync.Mutex
	activeSessions   int
	busy             bool // Simple Phase 1 lock: one command at a time
	lastActivityTime time.Time
}

func NewDaemonServer(m *plugin.Manager, proxy *DaemonUIProxy, rigVersion string, logger *slog.Logger) *DaemonServer {
	now := time.Now()
	return &DaemonServer{
		manager:          m,
		uiProxy:          proxy,
		logger:           logger,
		startTime:        now,
		rigVersion:       rigVersion,
		lastActivityTime: now,
	}
}

func (s *DaemonServer) Execute(stream apiv1.DaemonService_ExecuteServer) error {
	// Phase 1: TryLock. Only one active session allowed.
	s.mu.Lock()
	if s.busy {
		s.mu.Unlock()
		return status.Error(codes.ResourceExhausted, "daemon is busy with another command")
	}
	s.busy = true
	s.activeSessions++
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.busy = false
		s.activeSessions--
		s.lastActivityTime = time.Now()
		s.mu.Unlock()
		s.uiProxy.ClearActiveSession()
	}()

	// 1. Receive the first message which MUST be a CommandRequest
	msg, err := stream.Recv()
	if err != nil {
		return err
	}

	reqPayload, ok := msg.Payload.(*apiv1.DaemonServiceExecuteRequest_Command)
	if !ok {
		return status.Error(codes.InvalidArgument, "first message must be a CommandRequest")
	}
	req := reqPayload.Command

	// 2. Setup the UI Bridge for this session
	bridge := s.uiProxy.SetActiveSession(func(uiReq *apiv1.InteractResponse) error {
		return stream.Send(&apiv1.DaemonServiceExecuteResponse{
			Payload: &apiv1.DaemonServiceExecuteResponse_UiRequest{
				UiRequest: uiReq,
			},
		})
	})

	// 3. Start a goroutine to handle incoming UI responses from the CLI
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	go func() {
		for {
			m, err := stream.Recv()
			if err != nil {
				return
			}
			if uiResp, ok := m.Payload.(*apiv1.DaemonServiceExecuteRequest_UiResponse); ok {
				bridge.HandleResponse(uiResp.UiResponse)
			}
		}
	}()

	// 4. Execute the plugin command
	client, err := s.manager.GetCommandClient(ctx, req.PluginName)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to get plugin client: %v", err)
	}

	cmdStream, err := client.Execute(ctx, &apiv1.ExecuteRequest{
		Command: req.CommandName,
		Args:    req.Args,
		Flags:   req.Flags,
	})
	if err != nil {
		return status.Errorf(codes.Internal, "failed to execute plugin command: %v", err)
	}

	// 5. Proxy output chunks back to the CLI
	for {
		resp, err := cmdStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "plugin execution error: %v", err)
		}

		if err := stream.Send(&apiv1.DaemonServiceExecuteResponse{
			Payload: &apiv1.DaemonServiceExecuteResponse_Output{
				Output: resp,
			},
		}); err != nil {
			return err
		}

		if resp.Done {
			break
		}
	}

	return nil
}

func (s *DaemonServer) Status(ctx context.Context, _ *apiv1.DaemonServiceStatusRequest) (*apiv1.DaemonServiceStatusResponse, error) {
	s.mu.Lock()
	active := s.activeSessions
	s.mu.Unlock()

	return &apiv1.DaemonServiceStatusResponse{
		DaemonVersion:  s.rigVersion,
		UptimeSeconds:  int64(time.Since(s.startTime).Seconds()),
		ActiveSessions: int32(active),
		Pid:            int32(os.Getpid()),
		// Plugins: list of warm plugins could be added here in Phase 9
	}, nil
}
func (s *DaemonServer) Shutdown(ctx context.Context, req *apiv1.DaemonServiceShutdownRequest) (*apiv1.DaemonServiceShutdownResponse, error) {
	// Actual shutdown logic will be handled by the runner which calls Stop()
	return &apiv1.DaemonServiceShutdownResponse{Accepted: true}, nil
}

// LastActivityTime returns the time of the last session completion.
func (s *DaemonServer) LastActivityTime() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastActivityTime
}
