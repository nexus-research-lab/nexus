package provider

import (
	"errors"
	"regexp"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
)

var bearerTokenPattern = regexp.MustCompile(`(?i)(bearer\s+)[a-z0-9._\-]+`)

func sanitizeHTTPError(err error) error {
	if err == nil {
		return nil
	}
	return errors.New(sanitizeErrorMessage(err.Error()))
}

func sanitizeHTTPBody(body []byte, secrets ...string) string {
	value := sanitizeErrorMessage(string(body), secrets...)
	if len(value) > 400 {
		return value[:400] + "..."
	}
	return value
}

func sanitizedBodyPreview(body []byte, secrets ...string) string {
	return logx.PreviewText(sanitizeErrorMessage(string(body), secrets...), 2000)
}

func sanitizeErrorMessage(message string, secrets ...string) string {
	sanitized := bearerTokenPattern.ReplaceAllString(message, "${1}<redacted>")
	for _, marker := range []string{"Authorization", "authorization", "x-api-key", "api-key"} {
		sanitized = strings.ReplaceAll(sanitized, marker, "<redacted-header>")
	}
	for _, secret := range secrets {
		trimmed := strings.TrimSpace(secret)
		if trimmed == "" {
			continue
		}
		sanitized = strings.ReplaceAll(sanitized, trimmed, "<redacted>")
	}
	return strings.TrimSpace(sanitized)
}
