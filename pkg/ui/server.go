package ui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
	"google.golang.org/protobuf/types/known/emptypb"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

type readRequest struct {
	sensitive bool
	respCh    chan readResponse
}

type readResponse struct {
	value string
	err   error
}

// UIServer implements the UIService gRPC interface, allowing plugins to
// perform UI operations by calling back into the host.
type UIServer struct {
	apiv1.UnimplementedUIServiceServer
	coord *Coordinator
	in    io.Reader

	// Singleton reader infrastructure
	readCh chan readRequest
}

// NewUIServer creates a new UI server and starts the background stdin reader.
func NewUIServer() *UIServer {
	return NewUIServerWithReader(os.Stdin)
}

// NewUIServerWithReader creates a new UI server with a specific input reader.
func NewUIServerWithReader(in io.Reader) *UIServer {
	s := &UIServer{
		coord:  NewCoordinator(),
		in:     in,
		readCh: make(chan readRequest),
	}
	go s.runReader()
	return s
}

// runReader is the singleton background goroutine that owns the input reader.
// It ensures that only one read is active at a time and that no goroutines are leaked
// when RPCs are canceled.
func (s *UIServer) runReader() {
	reader := bufio.NewReader(s.in)
	for req := range s.readCh {
		var val string
		var err error

		if req.sensitive {
			// Password reading only works on real terminals (os.Stdin)
			if f, ok := s.in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
				var b []byte
				b, err = term.ReadPassword(int(f.Fd()))
				fmt.Println() // Move to next line after password entry
				val = string(b)
			} else {
				// Fallback for non-terminal readers (like tests)
				val, err = reader.ReadString('\n')
				val = strings.TrimSpace(val)
			}
		} else {
			val, err = reader.ReadString('\n')
			val = strings.TrimSpace(val)
		}

		// Send response. If the requester has already timed out or canceled,
		// they won't be listening, so we use a non-blocking send or 
		// just let it drop if the channel is closed.
		select {
		case req.respCh <- readResponse{value: val, err: err}:
		default:
			// Requester is gone, discard the input
		}
	}
}

func (s *UIServer) readInput(ctx context.Context, sensitive bool) (string, error) {
	respCh := make(chan readResponse, 1)
	req := readRequest{
		sensitive: sensitive,
		respCh:    respCh,
	}

	// Request a read from the singleton reader
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case s.readCh <- req:
	}

	// Wait for the response or cancellation
	select {
	case <-ctx.Done():
		// The background reader is still blocked on os.Stdin, but it's the
		// ONLY one. The next call to readInput will simply wait for it
		// to finish or send a new request.
		return "", ctx.Err()
	case res := <-respCh:
		return res.value, res.err
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

	input, err := s.readInput(ctx, req.Sensitive)
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

	input, err := s.readInput(ctx, false)
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
			input, err := s.readInput(ctx, false)
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
