package orchestration

import (
	"context"
	"os"

	"github.com/cockroachdb/errors"
)

// SecretResolver is responsible for resolving secret values based on their keys.
// This allows the orchestrator to fetch secrets from environment variables,
// keychains, or external secret managers (like AWS Secrets Manager, HashiCorp Vault)
// before passing them to the executing node.
type SecretResolver interface {
	// Resolve returns the plaintext value for the given secret key.
	// It should return an error if the secret cannot be found or accessed.
	Resolve(ctx context.Context, key string) (string, error)
}

// EnvSecretResolver resolves secrets by reading them from environment variables.
// This is primarily useful for local development or simple CI environments.
type EnvSecretResolver struct{}

// NewEnvSecretResolver creates a new EnvSecretResolver.
func NewEnvSecretResolver() *EnvSecretResolver {
	return &EnvSecretResolver{}
}

// Resolve looks up the key in the process environment.
func (e *EnvSecretResolver) Resolve(ctx context.Context, key string) (string, error) {
	val, ok := os.LookupEnv(key)
	if !ok {
		return "", errors.New("secret key not found in environment")
	}
	return val, nil
}
