package ui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	rigerrors "thoreinstein.com/rig/pkg/errors"
)

var (
	stdinReader *sharedReader
	stdinOnce   sync.Once
)

type sharedReader struct {
	readCh chan readRequest
}

func getStdinReader() *sharedReader {
	stdinOnce.Do(func() {
		stdinReader = &sharedReader{
			readCh: make(chan readRequest, 10),
		}
		go runReaderLoop(os.Stdin, stdinReader.readCh, context.Background())
	})
	return stdinReader
}

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
	ctx    context.Context
	cancel context.CancelFunc
	stop   sync.Once
	wg     sync.WaitGroup
}

// NewUIServer creates a new UI server and starts the background stdin reader.
func NewUIServer() *UIServer {
	return NewUIServerWithReader(os.Stdin)
}

// NewUIServerWithReader creates a new UI server with a specific input reader.
func NewUIServerWithReader(in io.Reader) *UIServer {
	ctx, cancel := context.WithCancel(context.Background())
	s := &UIServer{
		coord:  NewCoordinator(),
		in:     in,
		ctx:    ctx,
		cancel: cancel,
	}

	if in == os.Stdin {
		s.readCh = getStdinReader().readCh
	} else {
		ch := make(chan readRequest, 10)
		s.readCh = ch
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			runReaderLoop(in, ch, ctx)
		}()
	}

	return s
}

// Stop shuts down the UI server and its background reader.
func (s *UIServer) Stop() {
	s.stop.Do(func() {
		s.cancel()
		if s.in != os.Stdin {
			// For non-stdin readers (like tests), we close the channel.
			// The runReaderLoop will exit either via ctx.Done() or channel closure.
			if s.readCh != nil {
				close(s.readCh)
			}
			s.wg.Wait()
		}
	})
}

// runReaderLoop is the background loop that owns the input reader.
// It ensures that only one read is active at a time.
func runReaderLoop(in io.Reader, ch <-chan readRequest, ctx context.Context) {
	reader := bufio.NewReader(in)
	for {
		var req readRequest
		var ok bool

		select {
		case <-ctx.Done():
			return
		case req, ok = <-ch:
			if !ok {
				return
			}
		}

		// Use a separate goroutine for the blocking read so we can observe ctx.Done().
		// This prevents deadlocks when Stop() is called while a read is in progress.
		readDone := make(chan struct{})
		go func() {
			handleReadRequest(in, reader, req)
			close(readDone)
		}()

		select {
		case <-readDone:
			// Read finished normally
		case <-ctx.Done():
			// Shutdown requested. We return immediately to unblock callers.
			// Note: The goroutine above remains leaked until input is provided or EOF.
			return
		}
	}
}

func handleReadRequest(in io.Reader, reader *bufio.Reader, req readRequest) {
	var val string
	var err error

	if req.sensitive {
		// Password reading only works on real terminals (os.Stdin)
		if f, ok := in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
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
	// we use a non-blocking send.
	select {
	case req.respCh <- readResponse{value: val, err: err}:
	default:
		// Requester is gone, discard the input
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
		return "", rigerrors.Wrap(ctx.Err(), "input request canceled")
	case <-s.ctx.Done():
		return "", rigerrors.Wrap(s.ctx.Err(), "UI server stopped")
	case s.readCh <- req:
	}

	// Wait for the response or cancellation
	select {
	case <-ctx.Done():
		return "", rigerrors.Wrap(ctx.Err(), "input read timed out or canceled")
	case <-s.ctx.Done():
		return "", rigerrors.Wrap(s.ctx.Err(), "UI server stopped")
	case res := <-respCh:
		if res.err != nil {
			return "", rigerrors.Wrap(res.err, "failed to read input")
		}
		return res.value, nil
	}
}

// Prompt asks the user for a text input.
func (s *UIServer) Prompt(ctx context.Context, req *apiv1.PromptRequest) (*apiv1.PromptResponse, error) {
	unlock, err := s.coord.Lock(ctx)
	if err != nil {
		return nil, rigerrors.Wrap(err, "failed to acquire terminal lock for prompt")
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
		return nil, rigerrors.Wrap(err, "failed to acquire terminal lock for confirmation")
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
		return nil, rigerrors.Wrap(err, "failed to acquire terminal lock for selection")
	}
	defer unlock()

	if req.Multiselect {
		return nil, rigerrors.New("multiselect is not yet supported")
	}

	if len(req.Options) == 0 {
		return nil, rigerrors.New("select requires at least one option")
	}

	fmt.Println(req.Label)
	for i, opt := range req.Options {
		fmt.Printf("  %d) %s\n", i+1, opt)
	}

	for {
		select {
		case <-ctx.Done():
			return nil, rigerrors.Wrap(ctx.Err(), "selection canceled")
		case <-s.ctx.Done():
			return nil, rigerrors.Wrap(s.ctx.Err(), "UI server stopped")
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
func (s *UIServer) UpdateProgress(ctx context.Context, req *apiv1.UpdateProgressRequest) (*apiv1.UpdateProgressResponse, error) {
	// Acquire lock to ensure progress messages don't interleave with blocking UI or other messages.
	// We use a short timeout for progress updates to avoid stalling the plugin if the terminal is held.
	lockCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	unlock, err := s.coord.Lock(lockCtx)
	if err != nil {
		// If we can't get the lock quickly, we skip the update to keep the terminal consistent.
		return &apiv1.UpdateProgressResponse{}, nil
	}
	defer unlock()

	if req.Progress != nil && req.Progress.Message != "" {
		fmt.Fprintf(os.Stderr, "--> %s\n", req.Progress.Message)
	}
	return &apiv1.UpdateProgressResponse{}, nil
}

// HandlePrompt satisfies the daemon.UIHandler interface.
func (s *UIServer) HandlePrompt(ctx context.Context, req *apiv1.PromptRequest) (*apiv1.PromptResponse, error) {
	return s.Prompt(ctx, req)
}

// HandleConfirm satisfies the daemon.UIHandler interface.
func (s *UIServer) HandleConfirm(ctx context.Context, req *apiv1.ConfirmRequest) (*apiv1.ConfirmResponse, error) {
	return s.Confirm(ctx, req)
}

// HandleSelect satisfies the daemon.UIHandler interface.
func (s *UIServer) HandleSelect(ctx context.Context, req *apiv1.SelectRequest) (*apiv1.SelectResponse, error) {
	return s.Select(ctx, req)
}

// HandleProgress satisfies the daemon.UIHandler interface.
func (s *UIServer) HandleProgress(ctx context.Context, req *apiv1.ProgressUpdate) error {
	_, err := s.UpdateProgress(ctx, &apiv1.UpdateProgressRequest{Progress: req})
	return err
}
