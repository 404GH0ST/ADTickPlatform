package httpapi

import (
	"net/url"
	"strings"
)

// NormalizeInternalBaseURL accepts only absolute HTTP(S) origins for service-to-service clients.
func NormalizeInternalBaseURL(raw string) (string, bool) {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	if trimmed == "" {
		return "", false
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", false
	}
	if parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", false
	}
	return strings.TrimRight(parsed.String(), "/"), true
}
