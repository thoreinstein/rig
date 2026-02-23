package sdk

import (
	"context"
	"os"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

// UI is a high-level client for interacting with the Rig host's UIService.
// It handles lazy connection, environment variable resolution, and provide simplified wrappers.
type UI struct {
	mu       sync.Mutex
	endpoint string
	conn     *grpc.ClientConn
	client   apiv1.UIServiceClient
	dialOpts []grpc.DialOption
}

// UIOption is a functional option for configuring the UI client.
type UIOption func(*UI)

// WithHostEndpoint overrides the host's UDS endpoint.
func WithHostEndpoint(endpoint string) UIOption {
	return func(u *UI) {
		u.endpoint = endpoint
	}
}

// WithDialOptions adds custom gRPC dial options.
func WithDialOptions(opts ...grpc.DialOption) UIOption {
	return func(u *UI) {
		u.dialOpts = append(u.dialOpts, opts...)
	}
}

// NewUI creates a new UI client.
// It reads the host's endpoint from the RIG_HOST_ENDPOINT environment variable by default.
func NewUI(opts ...UIOption) *UI {
	u := &UI{
		endpoint: os.Getenv("RIG_HOST_ENDPOINT"),
	}
	for _, opt := range opts {
		opt(u)
	}
	return u
}

// Close closes the underlying gRPC connection.
func (u *UI) Close() error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.conn != nil {
		err := u.conn.Close()
		u.conn = nil
		u.client = nil
		return err
	}
	return nil
}

func (u *UI) connect() (apiv1.UIServiceClient, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.client != nil {
		return u.client, nil
	}

	if u.endpoint == "" {
		return nil, os.ErrInvalid // Or a more descriptive error
	}

	opts := append([]grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}, u.dialOpts...)

	conn, err := grpc.NewClient("unix://"+u.endpoint, opts...)
	if err != nil {
		return nil, err
	}

	u.conn = conn
	u.client = apiv1.NewUIServiceClient(conn)
	return u.client, nil
}

// PromptOption is a functional option for Prompt.
type PromptOption func(*apiv1.PromptRequest)

// WithDefault sets the default value for the prompt.
func WithDefault(v string) PromptOption {
	return func(r *apiv1.PromptRequest) {
		r.DefaultValue = v
	}
}

// WithPlaceholder sets the placeholder text for the prompt.
func WithPlaceholder(v string) PromptOption {
	return func(r *apiv1.PromptRequest) {
		r.Placeholder = v
	}
}

// Sensitive masks the user input for the prompt.
func Sensitive() PromptOption {
	return func(r *apiv1.PromptRequest) {
		r.Sensitive = true
	}
}

// Prompt asks the user for a text input.
func (u *UI) Prompt(ctx context.Context, label string, opts ...PromptOption) (string, error) {
	client, err := u.connect()
	if err != nil {
		return "", err
	}

	req := &apiv1.PromptRequest{Label: label}
	for _, opt := range opts {
		opt(req)
	}

	resp, err := client.Prompt(ctx, req)
	if err != nil {
		return "", mapError(err)
	}
	return resp.Value, nil
}

// Confirm asks the user for a yes/no confirmation.
func (u *UI) Confirm(ctx context.Context, label string, defaultValue bool) (bool, error) {
	client, err := u.connect()
	if err != nil {
		return false, err
	}

	resp, err := client.Confirm(ctx, &apiv1.ConfirmRequest{
		Label:        label,
		DefaultValue: defaultValue,
	})
	if err != nil {
		return false, mapError(err)
	}
	return resp.Confirmed, nil
}

// Select asks the user to choose from a list of options.
// It returns the index of the selected option.
func (u *UI) Select(ctx context.Context, label string, options []string) (int, error) {
	client, err := u.connect()
	if err != nil {
		return 0, err
	}

	resp, err := client.Select(ctx, &apiv1.SelectRequest{
		Label:   label,
		Options: options,
	})
	if err != nil {
		return 0, mapError(err)
	}

	if len(resp.SelectedIndices) == 0 {
		return 0, nil
	}
	return int(resp.SelectedIndices[0]), nil
}

// ProgressOption is a functional option for UpdateProgress.
type ProgressOption func(*apiv1.ProgressUpdate)

// WithProgressType sets the type of progress display.
func WithProgressType(t apiv1.ProgressUpdate_Type) ProgressOption {
	return func(u *apiv1.ProgressUpdate) {
		u.Type = t
	}
}

// WithProgressPercent sets the completion percentage.
func WithProgressPercent(p int) ProgressOption {
	return func(u *apiv1.ProgressUpdate) {
		u.Progress = int32(p)
	}
}

// WithIndeterminate sets whether the progress is indeterminate.
func WithIndeterminate(v bool) ProgressOption {
	return func(u *apiv1.ProgressUpdate) {
		u.Indeterminate = v
	}
}

// WithDone marks the progress as complete.
func WithDone() ProgressOption {
	return func(u *apiv1.ProgressUpdate) {
		u.Done = true
	}
}

// UpdateProgress provides real-time status updates for a long-running task.
func (u *UI) UpdateProgress(ctx context.Context, message string, opts ...ProgressOption) error {
	client, err := u.connect()
	if err != nil {
		return err
	}

	update := &apiv1.ProgressUpdate{Message: message}
	for _, opt := range opts {
		opt(update)
	}

	_, err = client.UpdateProgress(ctx, &apiv1.UpdateProgressRequest{
		Progress: update,
	})
	return mapError(err)
}
