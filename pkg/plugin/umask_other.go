//go:build !unix

package plugin

// setUmask is a no-op on non-unix platforms.
func setUmask(_ int) int { return 0 }
