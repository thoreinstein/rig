package vcs

// NewProvider creates a new VCS provider based on the provider name.
// If providerName is empty or "git", it returns a LocalProvider.
// This factory will be extended in Phase 3 to support plugin-based providers.
func NewProvider(providerName string, verbose bool) (Provider, error) {
	// Default to local git provider
	if providerName == "" || providerName == "git" {
		return NewLocalProvider(verbose), nil
	}

	// For now, fallback to local provider
	return NewLocalProvider(verbose), nil
}
