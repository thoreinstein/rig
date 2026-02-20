package daemon

import (
	"context"
	"io"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	"thoreinstein.com/rig/pkg/errors"
)

// UIHandler defines the interface for the CLI to handle UI requests from the daemon.
type UIHandler interface {
	HandlePrompt(ctx context.Context, req *apiv1.PromptRequest) (*apiv1.PromptResponse, error)
	HandleConfirm(ctx context.Context, req *apiv1.ConfirmRequest) (*apiv1.ConfirmResponse, error)
	HandleSelect(ctx context.Context, req *apiv1.SelectRequest) (*apiv1.SelectResponse, error)
	HandleProgress(ctx context.Context, req *apiv1.ProgressUpdate) error
}

// DaemonClient is a client for the Rig background daemon.
type DaemonClient struct {
	conn   *grpc.ClientConn
	client apiv1.DaemonServiceClient
}

// NewClient creates a new client connected to the daemon at the well-known socket path.
// It performs a connection check to ensure the daemon is reachable before returning.
func NewClient(ctx context.Context) (*DaemonClient, error) {
	path := SocketPath()

	// NewClient is lazy, so we dial the socket directly first to ensure it's up.
	d := net.Dialer{Timeout: 500 * time.Millisecond}
	testConn, err := d.DialContext(ctx, "unix", path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to daemon socket")
	}
	_ = testConn.Close()

	conn, err := grpc.NewClient("passthrough:///unix://"+path,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			return net.DialTimeout("unix", path, 500*time.Millisecond)
		}),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create daemon client")
	}

	return &DaemonClient{
		conn:   conn,
		client: apiv1.NewDaemonServiceClient(conn),
	}, nil
}

func (c *DaemonClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Status returns the current status of the daemon.
func (c *DaemonClient) Status(ctx context.Context) (*apiv1.DaemonServiceStatusResponse, error) {
	return c.client.Status(ctx, &apiv1.DaemonServiceStatusRequest{})
}

// Shutdown requests the daemon to shut down.
func (c *DaemonClient) Shutdown(ctx context.Context, force bool) error {
	_, err := c.client.Shutdown(ctx, &apiv1.DaemonServiceShutdownRequest{Force: force})
	return err
}

// ExecuteCommand runs a plugin command via the daemon, handling output and UI callbacks.
func (c *DaemonClient) ExecuteCommand(ctx context.Context, req *apiv1.CommandRequest, ui UIHandler, stdout, stderr io.Writer) error {
	stream, err := c.client.Execute(ctx)
	if err != nil {
		return errors.NewDaemonError("Execute", "failed to initiate command execution").WithCause(err)
	}

	// 1. Send initial command request
	err = stream.Send(&apiv1.DaemonServiceExecuteRequest{
		Payload: &apiv1.DaemonServiceExecuteRequest_Command{
			Command: req,
		},
	})
	if err != nil {
		return errors.NewDaemonError("Execute", "failed to send command request to daemon").WithCause(err)
	}

	// 2. Main loop: process output and UI requests
	var exitCode int32
	var gotDone bool

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrap(err, "daemon stream error")
		}

		switch p := resp.Payload.(type) {
		case *apiv1.DaemonServiceExecuteResponse_Output:
			out := p.Output
			if len(out.Stdout) > 0 {
				_, _ = stdout.Write(out.Stdout)
			}
			if len(out.Stderr) > 0 {
				_, _ = stderr.Write(out.Stderr)
			}
			if out.Done {
				exitCode = out.ExitCode
				gotDone = true
			}

		case *apiv1.DaemonServiceExecuteResponse_UiRequest:
			uiReq := p.UiRequest
			var uiResp *apiv1.InteractRequest
			var uiErr error

			switch up := uiReq.Payload.(type) {
			case *apiv1.InteractResponse_Prompt:
				res, err := ui.HandlePrompt(ctx, up.Prompt)
				uiErr = err
				uiResp = &apiv1.InteractRequest{
					ResponseTo: uiReq.Id,
					Payload:    &apiv1.InteractRequest_Prompt{Prompt: res},
				}
			case *apiv1.InteractResponse_Confirm:
				res, err := ui.HandleConfirm(ctx, up.Confirm)
				uiErr = err
				uiResp = &apiv1.InteractRequest{
					ResponseTo: uiReq.Id,
					Payload:    &apiv1.InteractRequest_Confirm{Confirm: res},
				}
			case *apiv1.InteractResponse_Select:
				res, err := ui.HandleSelect(ctx, up.Select)
				uiErr = err
				uiResp = &apiv1.InteractRequest{
					ResponseTo: uiReq.Id,
					Payload:    &apiv1.InteractRequest_Select{Select: res},
				}
			case *apiv1.InteractResponse_Progress:
				uiErr = ui.HandleProgress(ctx, up.Progress)
			}

			if uiErr != nil {
				return errors.Wrap(uiErr, "UI interaction failed")
			}

			// Send UI response back if applicable (Progress is fire-and-forget)
			if uiResp != nil {
				err = stream.Send(&apiv1.DaemonServiceExecuteRequest{
					Payload: &apiv1.DaemonServiceExecuteRequest_UiResponse{
						UiResponse: uiResp,
					},
				})
				if err != nil {
					return errors.Wrap(err, "failed to send UI response to daemon")
				}
			}

		case *apiv1.DaemonServiceExecuteResponse_Error:
			return errors.NewDaemonError("Execute", p.Error)
		}

		if gotDone {
			break
		}
	}

	if exitCode != 0 {
		// This is a plugin execution error, NOT a daemon transport error.
		// It should NOT trigger a fallback.
		return errors.Newf("plugin command exited with code %d", exitCode)
	}

	return nil
}
