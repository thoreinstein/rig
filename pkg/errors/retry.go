package errors

import (
	"context"
	"math"
	"math/rand/v2"
	"time"
)

// Retry configuration defaults.
const (
	DefaultMaxRetries = 3
	DefaultBaseDelay  = time.Second
	DefaultMaxDelay   = 30 * time.Second
	DefaultJitter     = 0.4 // Produces a multiplier range of [0.6, 1.4]
)

// RetryConfig holds configuration for retry behavior.
type RetryConfig struct {
	MaxRetries int           // Maximum number of retry attempts
	BaseDelay  time.Duration // Initial delay before first retry
	MaxDelay   time.Duration // Maximum delay between retries
	Jitter     float64       // Jitter factor (0.0 to 1.0)
}

// DefaultRetryConfig returns a RetryConfig with sensible defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: DefaultMaxRetries,
		BaseDelay:  DefaultBaseDelay,
		MaxDelay:   DefaultMaxDelay,
		Jitter:     DefaultJitter,
	}
}

// Retry executes fn with exponential backoff.
// It returns immediately if the error is not retryable or if ctx is cancelled.
// On success, returns nil. On failure after all retries, returns the last error.
func Retry(ctx context.Context, cfg RetryConfig, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		// Check context before each attempt
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return Wrapf(lastErr, "context cancelled after %d attempts", attempt)
			}
			return Wrap(err, "context cancelled before retry")
		}

		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		// Don't retry if the error is not retryable
		if !IsRetryable(lastErr) {
			return lastErr
		}

		// Don't wait after the last attempt
		if attempt == cfg.MaxRetries {
			break
		}

		delay := CalculateBackoff(cfg.BaseDelay, cfg.MaxDelay, attempt, cfg.Jitter)

		select {
		case <-ctx.Done():
			return Wrapf(lastErr, "context cancelled during retry backoff (attempt %d/%d)", attempt+1, cfg.MaxRetries)
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return Wrapf(lastErr, "failed after %d retries", cfg.MaxRetries)
}

// RetryWithResult executes fn and returns the result with exponential backoff.
// It returns immediately if the error is not retryable or if ctx is cancelled.
func RetryWithResult[T any](ctx context.Context, cfg RetryConfig, fn func() (T, error)) (T, error) {
	var lastErr error
	var result T

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		// Check context before each attempt
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return result, Wrapf(lastErr, "context cancelled after %d attempts", attempt)
			}
			return result, Wrap(err, "context cancelled before retry")
		}

		var err error
		result, err = fn()
		if err == nil {
			return result, nil
		}
		lastErr = err

		// Don't retry if the error is not retryable
		if !IsRetryable(lastErr) {
			return result, lastErr
		}

		// Don't wait after the last attempt
		if attempt == cfg.MaxRetries {
			break
		}

		delay := CalculateBackoff(cfg.BaseDelay, cfg.MaxDelay, attempt, cfg.Jitter)

		select {
		case <-ctx.Done():
			return result, Wrapf(lastErr, "context cancelled during retry backoff (attempt %d/%d)", attempt+1, cfg.MaxRetries)
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return result, Wrapf(lastErr, "failed after %d retries", cfg.MaxRetries)
}

// CalculateBackoff computes the delay for a retry attempt using exponential backoff with jitter.
// Formula: delay = min(base * 2^attempt, max) * (1 - jitter/2 + jitter*rand())
// For jitter=0.4, this produces a multiplier range of [0.8, 1.2], which is ±20% variation.
func CalculateBackoff(base, max time.Duration, attempt int, jitter float64) time.Duration {
	// Calculate exponential delay: base * 2^attempt
	expDelay := float64(base) * math.Pow(2, float64(attempt))

	// Cap at max delay
	if expDelay > float64(max) {
		expDelay = float64(max)
	}

	// Apply jitter: multiply by (1 - jitter/2 + jitter*rand())
	// For jitter=0.4, this gives range [0.8, 1.2] which is ±20%
	jitterMultiplier := 1.0 - jitter/2 + jitter*rand.Float64()
	delay := time.Duration(expDelay * jitterMultiplier)

	return delay
}
