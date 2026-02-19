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

// Prompt asks the user for a text input.
func (s *UIServer) Prompt(ctx context.Context, req *apiv1.PromptRequest) (*apiv1.PromptResponse, error) {
	unlock := s.coord.Lock()
	defer unlock()

	fmt.Printf("%s ", req.Label)
	if req.Placeholder != "" && req.DefaultValue == "" {
		fmt.Printf("[%s] ", req.Placeholder)
	} else if req.DefaultValue != "" {
		fmt.Printf("(default: %s) ", req.DefaultValue)
	}

	var input string
	var err error

	if req.Sensitive {
		var b []byte
		b, err = term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println() // Move to next line after password entry
		input = string(b)
	} else {
		reader := bufio.NewReader(os.Stdin)
		input, err = reader.ReadString('\n')
		input = strings.TrimSpace(input)
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
	unlock := s.coord.Lock()
	defer unlock()

	suffix := "[y/N]"
	if req.DefaultValue {
		suffix = "[Y/n]"
	}

	fmt.Printf("%s %s ", req.Label, suffix)

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}

	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" {
		return &apiv1.ConfirmResponse{Confirmed: req.DefaultValue}, nil
	}

	confirmed := strings.HasPrefix(input, "y")
	return &apiv1.ConfirmResponse{Confirmed: confirmed}, nil
}

// Select asks the user to choose from a list of options.
func (s *UIServer) Select(ctx context.Context, req *apiv1.SelectRequest) (*apiv1.SelectResponse, error) {
	unlock := s.coord.Lock()
	defer unlock()

	if len(req.Options) == 0 {
		return &apiv1.SelectResponse{}, nil
	}

	fmt.Println(req.Label)
	for i, opt := range req.Options {
		fmt.Printf("  %d) %s\n", i+1, opt)
	}

	for {
		fmt.Printf("Select (1-%d): ", len(req.Options))
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		input = strings.TrimSpace(input)
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

// UpdateProgress provides real-time status updates for a long-running task.
func (s *UIServer) UpdateProgress(ctx context.Context, req *apiv1.ProgressUpdate) (*emptypb.Empty, error) {
	// For now, just print to stderr. A more sophisticated implementation
	// would use a real spinner or progress bar library.
	if req.Message != "" {
		fmt.Fprintf(os.Stderr, "--> %s\n", req.Message)
	}
	return &emptypb.Empty{}, nil
}
