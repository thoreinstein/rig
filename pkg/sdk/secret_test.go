package sdk

import (
	"strings"
	"testing"

	"github.com/cockroachdb/errors"
)

func TestNewSecret(t *testing.T) {
	tests := []struct {
		name         string
		envValue     string
		opts         []SecretOption
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
			opts: []SecretOption{
				WithSecretHostEndpoint("/tmp/from-option.sock"),
			},
			wantEndpoint: "/tmp/from-option.sock",
		},
		{
			name:     "option_with_empty_env",
			envValue: "",
			opts: []SecretOption{
				WithSecretHostEndpoint("/tmp/from-option.sock"),
			},
			wantEndpoint: "/tmp/from-option.sock",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("RIG_HOST_ENDPOINT", tt.envValue)

			s := NewSecret(tt.opts...)
			if s.endpoint != tt.wantEndpoint {
				t.Errorf("endpoint = %q, want %q", s.endpoint, tt.wantEndpoint)
			}
		})
	}
}

func TestSecret_connect(t *testing.T) {
	tests := []struct {
		name       string
		endpoint   string
		wantErr    bool
		wantErrMsg string
		wantErrIs  error // use errors.Is check when non-nil
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
			name:       "https_endpoint_rejected",
			endpoint:   "https://localhost:443",
			wantErr:    true,
			wantErrMsg: "unix://",
		},
		{
			name:       "bare_host_rejected",
			endpoint:   "localhost:8080",
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
			// Ensure env var does not interfere.
			t.Setenv("RIG_HOST_ENDPOINT", "")

			s := NewSecret(WithSecretHostEndpoint(tt.endpoint))
			client, _, err := s.connect()

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

			// Clean up the connection.
			if err := s.Close(); err != nil {
				t.Errorf("Close() error: %v", err)
			}
		})
	}
}

func TestSecret_connect_returns_cached_client(t *testing.T) {
	t.Setenv("RIG_HOST_ENDPOINT", "")

	s := NewSecret(WithSecretHostEndpoint("/tmp/test.sock"))

	first, _, err := s.connect()
	if err != nil {
		t.Fatalf("first connect() error: %v", err)
	}

	second, _, err := s.connect()
	if err != nil {
		t.Fatalf("second connect() error: %v", err)
	}

	// The client interface value should be the same instance.
	if first != second {
		t.Error("connect() did not return cached client on second call")
	}

	if err := s.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}
}

func TestSecret_Close_noop(t *testing.T) {
	s := NewSecret()
	if err := s.Close(); err != nil {
		t.Errorf("Close() on unused Secret = %v, want nil", err)
	}
}

func TestSecret_Close_clears_state(t *testing.T) {
	t.Setenv("RIG_HOST_ENDPOINT", "")

	s := NewSecret(WithSecretHostEndpoint("/tmp/test.sock"))

	// Establish a connection.
	_, _, err := s.connect()
	if err != nil {
		t.Fatalf("connect() error: %v", err)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	// After Close, internal state should be cleared.
	s.mu.Lock()
	connNil := s.conn == nil
	clientNil := s.client == nil
	s.mu.Unlock()

	if !connNil {
		t.Error("conn not nil after Close()")
	}
	if !clientNil {
		t.Error("client not nil after Close()")
	}
}
