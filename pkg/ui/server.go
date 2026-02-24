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
	globalStdinReader *sharedReader
	stdinOnce         sync.Once
)

type sharedReader struct {
	reqCh chan readRequest
	done  chan struct{}
}

func getStdinReader() *sharedReader {
	stdinOnce.Do(func() {
		globalStdinReader = &sharedReader{
			reqCh: make(chan readRequest),
			done:  make(chan struct{}),
		}
		go globalStdinReader.runLoop(os.Stdin)
	})
	return globalStdinReader
}

// CloseStdinReader requests shutdown of the global stdin reader goroutine.
// Existing callers are not required to call this; if unused, the goroutine
// will continue running for the lifetime of the process, preserving
// existing behavior.
func CloseStdinReader() {
	// Synchronize with initialization. If getStdinReader is running, wait for it.
	// If it hasn't run yet, mark it as done so we don't start a reader just to close it.
	// Design consequence: subsequent getStdinReader calls will return the nil/stopped singleton.
	stdinOnce.Do(func() {})

	if globalStdinReader == nil {
		return
	}
	// Best-effort shutdown: closing done will unblock the runLoop.
	select {
	case <-globalStdinReader.done:
		// already closed
	default:
		close(globalStdinReader.done)
	}
}

func (s *sharedReader) runLoop(in io.Reader) {
	reader := bufio.NewReader(in)
	var bufferedRes *readResponse

	for {
		var req readRequest
		var ok bool

		select {
		case <-s.done:
			return
		case req, ok = <-s.reqCh:
			if !ok {
				return
			}
		}

		if bufferedRes != nil {
			// Delivery condition: both sides must be non-sensitive.
			if !req.sensitive && !bufferedRes.sensitive {
				select {
				case req.respCh <- *bufferedRes:
					bufferedRes = nil
					continue // Satisfied current request from buffer
				case <-req.abandoned:
					// Current caller also abandoned. Keep buffer for next time.
					continue
				case <-s.done:
					return
				}
			} else {
				// Sensitivity mismatch or sensitive request.
				// Drop the buffer and proceed to perform a fresh read for the current request.
				bufferedRes = nil
			}
		}

		res := s.performRead(in, reader, req.sensitive)

		select {
		case req.respCh <- res:
			// Delivered successfully.
		case <-req.abandoned:
			// Caller stopped waiting (timeout/cancel). Only buffer if not sensitive.
			if !req.sensitive {
				bufferedRes = &res
			}
		case <-s.done:
			return
		}
	}
}

func (s *sharedReader) performRead(in io.Reader, reader *bufio.Reader, sensitive bool) readResponse {
	var val string
	var err error

	if sensitive {
		if f, ok := in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
			// Drain any buffered bytes from the reader to ensure FD synchronization
			if n := reader.Buffered(); n > 0 {
				_, _ = reader.Discard(n)
			}
			var b []byte
			b, err = term.ReadPassword(int(f.Fd()))
			fmt.Println()
			val = string(b)
		} else {
			val, err = reader.ReadString('\n')
			val = strings.TrimSpace(val)
		}
	} else {
		val, err = reader.ReadString('\n')
		val = strings.TrimSpace(val)
	}

	return readResponse{value: val, err: err, sensitive: sensitive}
}

type readRequest struct {
	sensitive bool
	respCh    chan readResponse
	abandoned <-chan struct{} // Closed by caller if they stop waiting
}

type readResponse struct {
	value     string
	err       error
	sensitive bool
}

// UIServer implements the UIService gRPC interface, allowing plugins to
// perform UI operations by calling back into the host.
type UIServer struct {
	apiv1.UnimplementedUIServiceServer
	coord *Coordinator
	in    io.Reader

	// reader infrastructure
	reqCh  chan readRequest
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
// For reliable shutdown and to prevent goroutine leaks, the provided reader
// should ideally implement io.Closer (e.g. *os.File or io.PipeReader).
func NewUIServerWithReader(in io.Reader) *UIServer {
	ctx, cancel := context.WithCancel(context.Background())
	s := &UIServer{
		coord:  NewCoordinator(),
		in:     in,
		ctx:    ctx,
		cancel: cancel,
	}

	if in != os.Stdin {
		s.reqCh = make(chan readRequest)
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.runLocalReader()
		}()
	}

	return s
}

// runLocalReader is used for non-stdin readers (like tests).
func (s *UIServer) runLocalReader() {
	reader := bufio.NewReader(s.in)
	var bufferedRes *readResponse

	for {
		var req readRequest
		var ok bool

		select {
		case <-s.ctx.Done():
			return
		case req, ok = <-s.reqCh:
			if !ok {
				return
			}
		}

		if bufferedRes != nil {
			// Delivery condition: both sides must be non-sensitive.
			if !req.sensitive && !bufferedRes.sensitive {
				select {
				case req.respCh <- *bufferedRes:
					bufferedRes = nil
					continue
				case <-req.abandoned:
					// Current caller also abandoned? Keep buffer.
					continue
				case <-s.ctx.Done():
					return
				}
			} else {
				// Sensitivity mismatch or sensitive request.
				// Drop the buffer and proceed to perform a fresh read.
				bufferedRes = nil
			}
		}

		res := readResponse{sensitive: req.sensitive}
		if req.sensitive {
			if f, ok := s.in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
				// Drain any buffered bytes from the reader to ensure FD synchronization
				if n := reader.Buffered(); n > 0 {
					_, _ = reader.Discard(n)
				}
				var b []byte
				b, res.err = term.ReadPassword(int(f.Fd()))
				fmt.Println()
				res.value = string(b)
			} else {
				res.value, res.err = reader.ReadString('\n')
				res.value = strings.TrimSpace(res.value)
			}
		} else {
			res.value, res.err = reader.ReadString('\n')
			res.value = strings.TrimSpace(res.value)
		}

		select {
		case req.respCh <- res:
			// Success
		case <-req.abandoned:
			// Caller gone. Only buffer if not sensitive.
			if !req.sensitive {
				bufferedRes = &res
			}
		case <-s.ctx.Done():
			// Server stopping.
			return
		}
	}
}

// Stop shuts down the UI server and its background reader.
func (s *UIServer) Stop() {
	s.stop.Do(func() {
		s.cancel()
		if s.in != os.Stdin {
			if closer, ok := s.in.(io.Closer); ok {
				// Closing the reader unblocks any pending Read calls in runLocalReader.
				_ = closer.Close()
			}
			// We don't close s.reqCh here to avoid panics in readInput if it
			// attempts to send after Stop is called. The runLocalReader goroutine
			// will exit via s.ctx.Done().
			s.wg.Wait()
		}
	})
}

func (s *UIServer) readInput(ctx context.Context, sensitive bool) (string, error) {
	// respCh MUST be unbuffered so the reader can detect abandonment via the
	// abandoned channel when the requester stops waiting (e.g., on timeout or cancel).
	respCh := make(chan readResponse)
	abandoned := make(chan struct{})

	req := readRequest{
		sensitive: sensitive,
		respCh:    respCh,
		abandoned: abandoned,
	}

	var reqCh chan<- readRequest
	var readerDone <-chan struct{}
	if s.in == os.Stdin {
		reader := getStdinReader()
		if reader == nil {
			return "", rigerrors.New("stdin reader not initialized or stopped")
		}
		reqCh = reader.reqCh
		readerDone = reader.done
	} else {
		reqCh = s.reqCh
	}

	// Request a read
	select {
	case <-ctx.Done():
		return "", rigerrors.Wrap(ctx.Err(), "input request canceled")
	case <-s.ctx.Done():
		return "", rigerrors.Wrap(s.ctx.Err(), "UI server stopped")
	case <-readerDone:
		return "", rigerrors.New("stdin reader stopped")
	case reqCh <- req:
	}

	// Wait for the response
	select {
	case <-ctx.Done():
		close(abandoned)
		return "", rigerrors.Wrap(ctx.Err(), "input read timed out or canceled")
	case <-s.ctx.Done():
		close(abandoned)
		return "", rigerrors.Wrap(s.ctx.Err(), "UI server stopped")
	case <-readerDone:
		close(abandoned)
		return "", rigerrors.New("stdin reader stopped")
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
