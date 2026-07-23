package configuration

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestSanitizeValueRedactsNestedAndCamelCaseSecrets(t *testing.T) {
	input := map[string]any{
		"DatabaseURL":           "postgres://user:password@example.com/nexus",
		"MainAgentSystemPrompt": "hidden prompt",
		"mcp_servers": map[string]any{
			"remote": map[string]any{
				"headers": map[string]any{"Authorization": "Bearer top-secret"},
				"env":     map[string]any{"SERVICE_API_KEY": "api-secret"},
				"url":     "https://token-user@example.com/mcp?api-version=1&token=url-secret#fragment-secret",
			},
		},
		"display_name": "safe",
	}
	payload, err := json.Marshal(sanitizeValue(input))
	if err != nil {
		t.Fatalf("marshal sanitized value: %v", err)
	}
	text := string(payload)
	for _, secret := range []string{
		"postgres://", "hidden prompt", "Bearer top-secret", "api-secret",
		"token-user", "url-secret", "fragment-secret",
	} {
		if strings.Contains(text, secret) {
			t.Fatalf("sanitized payload leaked %q: %s", secret, text)
		}
	}
	if !strings.Contains(text, `"display_name":"safe"`) {
		t.Fatalf("safe fields should remain visible: %s", text)
	}
	if !strings.Contains(text, `"configured":true`) {
		t.Fatalf("secret presence should remain inspectable: %s", text)
	}
}

func TestRedactInputSecretsFromExecutionError(t *testing.T) {
	input := json.RawMessage(`{"credentials":{"token":"secret-value"},"name":"safe"}`)
	err := redactInputSecrets(errors.New("remote rejected secret-value for safe"), input)
	if strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("error leaked input secret: %v", err)
	}
	if !strings.Contains(err.Error(), "safe") {
		t.Fatalf("non-secret context should remain: %v", err)
	}
}

func TestRevisionIgnoresSecretContentsButTracksConfigurationShape(t *testing.T) {
	first, err := revisionFor(map[string]any{"token": "one", "enabled": true})
	if err != nil {
		t.Fatal(err)
	}
	second, err := revisionFor(map[string]any{"token": "two", "enabled": true})
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("secret value must not be encoded into externally visible revision: %s != %s", first, second)
	}
	third, err := revisionFor(map[string]any{"token": "", "enabled": true})
	if err != nil {
		t.Fatal(err)
	}
	if first == third {
		t.Fatal("revision must track whether a secret is configured")
	}
}
