package daemon

import (
	"context"
	"log/slog"
	"sync"

	"github.com/google/uuid"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	"thoreinstein.com/rig/pkg/errors"
)

// UIBridge defines the interface for sending UI requests to the CLI and receiving responses.
type UIBridge interface {
	SendRequest(resp *apiv1.InteractResponse) error
	RegisterResponse(id string) (chan *apiv1.InteractRequest, func())
	WaitResponse(ctx context.Context, id string, ch chan *apiv1.InteractRequest) (*apiv1.InteractRequest, error)
}

// sessionBridge manages the communication for a single active CLI session.
type sessionBridge struct {
	send    func(*apiv1.InteractResponse) error
	pending map[string]chan *apiv1.InteractRequest
	mu      sync.Mutex
}

func newSessionBridge(send func(*apiv1.InteractResponse) error) *sessionBridge {
	return &sessionBridge{
		send:    send,
		pending: make(map[string]chan *apiv1.InteractRequest),
	}
}

func (b *sessionBridge) SendRequest(resp *apiv1.InteractResponse) error {
	return b.send(resp)
}

// RegisterResponse installs a channel to receive the UI response and returns a cleanup function.
// MUST be called before SendRequest to avoid race conditions.
func (b *sessionBridge) RegisterResponse(id string) (chan *apiv1.InteractRequest, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan *apiv1.InteractRequest, 1)
	b.pending[id] = ch

	cleanup := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.pending, id)
	}

	return ch, cleanup
}

// WaitResponse blocks until the response for the given ID is received or the context is canceled.
func (b *sessionBridge) WaitResponse(ctx context.Context, id string, ch chan *apiv1.InteractRequest) (*apiv1.InteractRequest, error) {
	select {
	case res := <-ch:
		return res, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
func (b *sessionBridge) HandleResponse(res *apiv1.InteractRequest) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ch, ok := b.pending[res.ResponseTo]; ok {
		select {
		case ch <- res:
		default:
		}
	}
}

// DaemonUIProxy implements apiv1.UIServiceServer and proxies calls to the active session.
type DaemonUIProxy struct {
	apiv1.UnimplementedUIServiceServer
	mu            sync.RWMutex
	activeSession *sessionBridge
}

func NewDaemonUIProxy() *DaemonUIProxy {
	return &DaemonUIProxy{}
}

func (p *DaemonUIProxy) SetActiveSession(send func(*apiv1.InteractResponse) error) *sessionBridge {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.activeSession = newSessionBridge(send)
	return p.activeSession
}

func (p *DaemonUIProxy) ClearActiveSession() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.activeSession = nil
}

func (p *DaemonUIProxy) getBridge() (*sessionBridge, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.activeSession == nil {
		return nil, errors.New("no active CLI session to handle UI request")
	}
	return p.activeSession, nil
}

func (p *DaemonUIProxy) Prompt(ctx context.Context, req *apiv1.PromptRequest) (*apiv1.PromptResponse, error) {
	bridge, err := p.getBridge()
	if err != nil {
		return nil, err
	}

	id := uuid.New().String()
	respCh, cleanup := bridge.RegisterResponse(id)
	defer cleanup()

	err = bridge.SendRequest(&apiv1.InteractResponse{
		Id: id,
		Payload: &apiv1.InteractResponse_Prompt{
			Prompt: req,
		},
	})
	if err != nil {
		return nil, err
	}

	resp, err := bridge.WaitResponse(ctx, id, respCh)
	if err != nil {
		return nil, err
	}
	if payload, ok := resp.Payload.(*apiv1.InteractRequest_Prompt); ok {
		return payload.Prompt, nil
	}
	return nil, errors.New("unexpected response type for Prompt")
}

func (p *DaemonUIProxy) Confirm(ctx context.Context, req *apiv1.ConfirmRequest) (*apiv1.ConfirmResponse, error) {
	bridge, err := p.getBridge()
	if err != nil {
		return nil, err
	}

	id := uuid.New().String()
	respCh, cleanup := bridge.RegisterResponse(id)
	defer cleanup()

	err = bridge.SendRequest(&apiv1.InteractResponse{
		Id: id,
		Payload: &apiv1.InteractResponse_Confirm{
			Confirm: req,
		},
	})
	if err != nil {
		return nil, err
	}

	resp, err := bridge.WaitResponse(ctx, id, respCh)
	if err != nil {
		return nil, err
	}
	if payload, ok := resp.Payload.(*apiv1.InteractRequest_Confirm); ok {
		return payload.Confirm, nil
	}
	return nil, errors.New("unexpected response type for Confirm")
}

func (p *DaemonUIProxy) Select(ctx context.Context, req *apiv1.SelectRequest) (*apiv1.SelectResponse, error) {
	bridge, err := p.getBridge()
	if err != nil {
		return nil, err
	}

	id := uuid.New().String()
	respCh, cleanup := bridge.RegisterResponse(id)
	defer cleanup()

	err = bridge.SendRequest(&apiv1.InteractResponse{
		Id: id,
		Payload: &apiv1.InteractResponse_Select{
			Select: req,
		},
	})
	if err != nil {
		return nil, err
	}

	resp, err := bridge.WaitResponse(ctx, id, respCh)
	if err != nil {
		return nil, err
	}
	if payload, ok := resp.Payload.(*apiv1.InteractRequest_Select); ok {
		return payload.Select, nil
	}
	return nil, errors.New("unexpected response type for Select")
}

func (p *DaemonUIProxy) UpdateProgress(ctx context.Context, req *apiv1.UpdateProgressRequest) (*apiv1.UpdateProgressResponse, error) {
	bridge, _ := p.getBridge()
	if bridge == nil {
		// Non-blocking status updates can be ignored if no session is active
		return &apiv1.UpdateProgressResponse{}, nil
	}

	id := uuid.New().String()
	if err := bridge.SendRequest(&apiv1.InteractResponse{
		Id: id,
		Payload: &apiv1.InteractResponse_Progress{
			Progress: req.Progress,
		},
	}); err != nil {
		slog.Debug("Failed to send UI progress update to session", "id", id, "error", err)
	}

	return &apiv1.UpdateProgressResponse{}, nil
}
