package events

import (
	"encoding/json"
	"regexp"
	"strings"

	"thoreinstein.com/rig/pkg/config"
)

var (
	// patterns for scanning values in strings
	bearerRegex     = regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9\-\._~+/]+=*`)
	tokenParamRegex = regexp.MustCompile(`(?i)(token|access_token|refresh_token|api_key|apikey|secret|client_secret|password|credential|session_token|auth)=[^&\s]+`)
	urlCredsRegex   = regexp.MustCompile(`(?i)[a-zA-Z][a-zA-Z0-9+.\-]*://[^:]+:[^@]+@`)
	basicAuthRegex  = regexp.MustCompile(`(?i)basic\s+[a-zA-Z0-9+/]+=*`)
	awsKeyRegex     = regexp.MustCompile(`AKIA[0-9A-Z]{16}`)
	ghTokenRegex    = regexp.MustCompile(`(ghp_[a-zA-Z0-9]{36}|github_pat_[a-zA-Z0-9_]{22,})`)
	aiKeyRegex      = regexp.MustCompile(`(sk-ant-[a-zA-Z0-9\-_]{20,}|sk-proj-[a-zA-Z0-9\-_]{20,})`)
	pemBlockRegex   = regexp.MustCompile(`-----BEGIN [A-Z ]+-----[\s\S]*?-----END [A-Z ]+-----`)
)

const redactionString = "[REDACTED]"

// RedactMessage replaces sensitive patterns in a string with [REDACTED].
func RedactMessage(msg string) string {
	if msg == "" {
		return ""
	}

	// Redact PEM private key blocks (must run before shorter patterns)
	msg = pemBlockRegex.ReplaceAllString(msg, redactionString)

	// Redact Bearer tokens
	msg = bearerRegex.ReplaceAllString(msg, "Bearer "+redactionString)

	// Redact Basic auth
	msg = basicAuthRegex.ReplaceAllString(msg, "Basic "+redactionString)

	// Redact common query/form params
	msg = tokenParamRegex.ReplaceAllStringFunc(msg, func(match string) string {
		parts := strings.SplitN(match, "=", 2)
		if len(parts) == 2 {
			return parts[0] + "=" + redactionString
		}
		return match
	})

	// Redact URL credentials
	msg = urlCredsRegex.ReplaceAllStringFunc(msg, func(match string) string {
		// match is e.g. "https://user:pass@"
		schemeEnd := strings.Index(match, "://")
		if schemeEnd == -1 {
			return redactionString + "@"
		}
		return match[:schemeEnd+3] + redactionString + "@"
	})

	// Redact cloud provider credential patterns
	msg = awsKeyRegex.ReplaceAllString(msg, redactionString)
	msg = ghTokenRegex.ReplaceAllString(msg, redactionString)
	msg = aiKeyRegex.ReplaceAllString(msg, redactionString)

	return msg
}

// RedactMetadata takes a JSON metadata string and redacts any values associated
// with keys that are considered sensitive. Walks nested objects and arrays.
func RedactMetadata(metadata string) string {
	if metadata == "" || metadata == "{}" {
		return metadata
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(metadata), &m); err != nil {
		// Not valid JSON, try simple string redaction as fallback
		return RedactMessage(metadata)
	}

	if !redactValue(m) {
		return metadata
	}

	b, err := json.Marshal(m)
	if err != nil {
		return `{"_redaction_error":"failed to re-serialize"}`
	}
	return string(b)
}

// redactValue recursively walks a parsed JSON value, redacting sensitive keys
// and scanning all string leaves for credential patterns. Returns true if any
// value was modified.
func redactValue(v any) bool {
	changed := false
	switch val := v.(type) {
	case map[string]any:
		for k, child := range val {
			if config.IsSensitiveKey(k) {
				val[k] = redactionString
				changed = true
				continue
			}
			switch c := child.(type) {
			case string:
				newS := RedactMessage(c)
				if newS != c {
					val[k] = newS
					changed = true
				}
			case map[string]any, []any:
				if redactValue(c) {
					changed = true
				}
			}
		}
	case []any:
		for i, item := range val {
			switch c := item.(type) {
			case string:
				newS := RedactMessage(c)
				if newS != c {
					val[i] = newS
					changed = true
				}
			case map[string]any, []any:
				if redactValue(c) {
					changed = true
				}
			}
		}
	}
	return changed
}
