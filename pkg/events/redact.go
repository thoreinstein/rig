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
	tokenParamRegex = regexp.MustCompile(`(?i)(token|api_key|secret|password)=[^&\s]+`)
	urlCredsRegex   = regexp.MustCompile(`(?i)[a-z]+://[^:]+:[^@]+@`)
)

const redactionString = "[REDACTED]"

// RedactMessage replaces sensitive patterns in a string with [REDACTED].
func RedactMessage(msg string) string {
	if msg == "" {
		return ""
	}

	// Redact Bearer tokens
	msg = bearerRegex.ReplaceAllString(msg, "Bearer "+redactionString)

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

	return msg
}

// RedactMetadata takes a JSON metadata string and redacts any values associated
// with keys that are considered sensitive.
func RedactMetadata(metadata string) string {
	if metadata == "" || metadata == "{}" {
		return metadata
	}

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(metadata), &m); err != nil {
		// Not valid JSON, try simple string redaction as fallback
		return RedactMessage(metadata)
	}

	redacted := false
	for k, v := range m {
		if config.IsSensitiveKey(k) {
			m[k] = redactionString
			redacted = true
			continue
		}

		// If value is a string, also scan for patterns
		if s, ok := v.(string); ok {
			newS := RedactMessage(s)
			if newS != s {
				m[k] = newS
				redacted = true
			}
		}
	}

	if !redacted {
		return metadata
	}

	b, err := json.Marshal(m)
	if err != nil {
		return metadata
	}
	return string(b)
}
