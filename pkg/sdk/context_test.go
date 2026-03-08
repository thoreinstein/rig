package sdk

import (
	"strings"
	"testing"

	"github.com/cockroachdb/errors"
)

func TestNewContext(t *testing.T) {
	tests := []struct {
		name         string
		envValue     string
		opts         []ContextOption
		wantEndpoint string
	}{
		{
			name:         "no_env_no_opts",
			envValue:     "",
			wantEndpoint: "",
		},
		{
			name:         "env_set",
			envValue:     "/tmp/rig-host.sock",
			wantEndpoint: "/tmp/rig-host.sock",
		},
		{
			name:     "option_overrides_env",
			envValue: "/tmp/from-env.sock",
			opts: []ContextOption{
				WithContextHostEndpoint("/tmp/from-option.sock"),
			},
			wantEndpoint: "/tmp/from-option.sock",
		},
		{
			name:     "option_with_empty_env",
			envValue: "",
			opts: []ContextOption{
				WithContextHostEndpoint("/tmp/from-option.sock"),
			},
			wantEndpoint: "/tmp/from-option.sock",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("RIG_HOST_ENDPOINT", tt.envValue)

			c := NewContext(tt.opts...)
			if c.endpoint != tt.wantEndpoint {
				t.Errorf("endpoint = %q, want %q", c.endpoint, tt.wantEndpoint)
			}
		})
	}
}

func TestContext_connect(t *testing.T) {
	tests := []struct {
		name       string
		endpoint   string
		wantErr    bool
		wantErrMsg string
		wantErrIs  error
		wantClient bool
	}{
		{
			name:      "empty_endpoint",
			endpoint:  "",
			wantErr:   true,
			wantErrIs: ErrNoEndpoint,
		},
		{
			name:       "tcp_endpoint_rejected",
			endpoint:   "tcp://localhost:8080",
			wantErr:    true,
			wantErrMsg: "unix://",
		},
		{
			name:       "http_endpoint_rejected",
			endpoint:   "http://localhost:8080",
			wantErr:    true,
			wantErrMsg: "unix://",
		},
		{
			name:       "absolute_path_auto_prefixed",
			endpoint:   "/tmp/test.sock",
			wantErr:    false,
			wantClient: true,
		},
		{
			name:       "relative_path_auto_prefixed",
			endpoint:   "./test.sock",
			wantErr:    false,
			wantClient: true,
		},
		{
			name:       "explicit_unix_scheme",
			endpoint:   "unix:///tmp/test.sock",
			wantErr:    false,
			wantClient: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("RIG_HOST_ENDPOINT", "")

			c := NewContext(WithContextHostEndpoint(tt.endpoint))
			client, _, err := c.connect()

			if tt.wantErr {
				if err == nil {
					t.Fatal("connect() returned nil error, want error")
				}
				if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
					t.Errorf("connect() error = %v, want errors.Is(%v)", err, tt.wantErrIs)
				}
				if tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("connect() error = %q, want substring %q", err.Error(), tt.wantErrMsg)
				}
				if client != nil {
					t.Error("connect() returned non-nil client on error")
				}
				return
			}

			if err != nil {
				t.Fatalf("connect() unexpected error: %v", err)
			}
			if tt.wantClient && client == nil {
				t.Error("connect() returned nil client, want non-nil")
			}

			if err := c.Close(); err != nil {
				t.Errorf("Close() error: %v", err)
			}
		})
	}
}

func TestContext_Close_noop(t *testing.T) {
	c := NewContext()
	if err := c.Close(); err != nil {
		t.Errorf("Close() on unused Context = %v, want nil", err)
	}
}

func TestContext_Close_clears_state(t *testing.T) {
	t.Setenv("RIG_HOST_ENDPOINT", "")

	c := NewContext(WithContextHostEndpoint("/tmp/test.sock"))

	_, _, err := c.connect()
	if err != nil {
		t.Fatalf("connect() error: %v", err)
	}

	if err := c.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	c.mu.Lock()
	connNil := c.conn == nil
	clientNil := c.client == nil
	c.mu.Unlock()

	if !connNil {
		t.Error("conn not nil after Close()")
	}
	if !clientNil {
		t.Error("client not nil after Close()")
	}
}
