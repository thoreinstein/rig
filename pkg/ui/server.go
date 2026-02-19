package ui

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
	"google.golang.org/protobuf/types/known/emptypb"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

// UIServer implements the UIService gRPC interface, allowing plugins to
// perform UI operations by calling back into the host.
type UIServer struct {
	apiv1.UnimplementedUIServiceServer
	coord *Coordinator
}

// NewUIServer creates a new UI server.
func NewUIServer() *UIServer {
	return &UIServer{
		coord: NewCoordinator(),
	}
}

// readStringLine performs a cancellation-aware read from Stdin.
// Since os.Stdin.Read is not natively cancellable, we run it in a goroutine.
// Note: This does not stop the background read if the context is cancelled, 
// but it unblocks the RPC handler immediately.
func (s *UIServer) readStringLine(ctx context.Context) (string, error) {
	type result struct {
		s   string
		err error
	}
	resCh := make(chan result, 1)

	go func() {
		reader := bufio.NewReader(os.Stdin)
		s, err := reader.ReadString('\n')
		resCh <- result{s: strings.TrimSpace(s), err: err}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case res := <-resCh:
		return res.s, res.err
	}
}

// readPassword performs a cancellation-aware password read from Stdin.
func (s *UIServer) readPassword(ctx context.Context) (string, error) {
	type result struct {
		s   string
		err error
	}
	resCh := make(chan result, 1)

	go func() {
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println() // Move to next line after password entry
		resCh <- result{s: string(b), err: err}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case res := <-resCh:
		return res.s, res.err
	}
}

// Prompt asks the user for a text input.
func (s *UIServer) Prompt(ctx context.Context, req *apiv1.PromptRequest) (*apiv1.PromptResponse, error) {
	unlock, err := s.coord.Lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()

	fmt.Printf("%s ", req.Label)
	if req.Placeholder != "" && req.DefaultValue == "" {
		fmt.Printf("[%s] ", req.Placeholder)
	} else if req.DefaultValue != "" {
		fmt.Printf("(default: %s) ", req.DefaultValue)
	}

	var input string
	if req.Sensitive {
		input, err = s.readPassword(ctx)
	} else {
		input, err = s.readStringLine(ctx)
	}

	if err != nil {
		return nil, err
	}

	if input == "" {
		input = req.DefaultValue
	}

	return &apiv1.PromptResponse{Value: input}, nil
}

// Confirm asks the user for a yes/no confirmation.
func (s *UIServer) Confirm(ctx context.Context, req *apiv1.ConfirmRequest) (*apiv1.ConfirmResponse, error) {
	unlock, err := s.coord.Lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()

	suffix := "[y/N]"
	if req.DefaultValue {
		suffix = "[Y/n]"
	}

	fmt.Printf("%s %s ", req.Label, suffix)

	input, err := s.readStringLine(ctx)
	if err != nil {
		return nil, err
	}

	input = strings.ToLower(input)
	if input == "" {
		return &apiv1.ConfirmResponse{Confirmed: req.DefaultValue}, nil
	}

	confirmed := strings.HasPrefix(input, "y")
	return &apiv1.ConfirmResponse{Confirmed: confirmed}, nil
}

// Select asks the user to choose from a list of options.
func (s *UIServer) Select(ctx context.Context, req *apiv1.SelectRequest) (*apiv1.SelectResponse, error) {
	unlock, err := s.coord.Lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()

	if len(req.Options) == 0 {
		return &apiv1.SelectResponse{}, nil
	}

	fmt.Println(req.Label)
	for i, opt := range req.Options {
		fmt.Printf("  %d) %s\n", i+1, opt)
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			fmt.Printf("Select (1-%d): ", len(req.Options))
			input, err := s.readStringLine(ctx)
			if err != nil {
				return nil, err
			}

			if input == "" {
				continue
			}

			var idx int
			_, err = fmt.Sscanf(input, "%d", &idx)
			if err != nil || idx < 1 || idx > len(req.Options) {
				fmt.Println("Invalid selection.")
				continue
			}

			return &apiv1.SelectResponse{
				SelectedIndices: []uint32{uint32(idx - 1)},
			}, nil
		}
	}
}

// UpdateProgress provides real-time status updates for a long-running task.
func (s *UIServer) UpdateProgress(ctx context.Context, req *apiv1.ProgressUpdate) (*emptypb.Empty, error) {
	// Progress updates are non-blocking and do not require the coordinator lock.
	if req.Message != "" {
		fmt.Fprintf(os.Stderr, "--> %s\n", req.Message)
	}
	return &emptypb.Empty{}, nil
}
