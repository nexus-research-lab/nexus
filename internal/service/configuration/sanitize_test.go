package configuration

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
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

func TestSanitizeValueRedactsWholeAgentMCPServers(t *testing.T) {
	const (
		argumentSecret    = "mcp-argument-secret"
		environmentSecret = "mcp-environment-secret"
		headerSecret      = "mcp-header-secret"
	)
	agentValue := protocol.Agent{
		AgentID: "agent-safe",
		Name:    "Safe Agent",
		Options: protocol.Options{
			MCPServers: map[string]any{
				"stdio": map[string]any{
					"command": "safe-command",
					"args":    []string{"--credential", argumentSecret},
					"env":     map[string]string{"CUSTOM_VALUE": environmentSecret},
				},
				"http": map[string]any{
					"url":     "https://mcp.example.com",
					"headers": map[string]string{"X-Custom-Value": headerSecret},
				},
			},
		},
	}

	payload, err := json.Marshal(sanitizeValue(agentValue))
	if err != nil {
		t.Fatalf("marshal sanitized Agent: %v", err)
	}
	text := string(payload)
	for _, secret := range []string{argumentSecret, environmentSecret, headerSecret} {
		if strings.Contains(text, secret) {
			t.Fatalf("sanitized Agent leaked MCP secret %q: %s", secret, text)
		}
	}
	for _, privateShape := range []string{`"args"`, `"env"`, `"headers"`} {
		if strings.Contains(text, privateShape) {
			t.Fatalf("sanitized Agent exposed MCP server shape %s: %s", privateShape, text)
		}
	}
	if !strings.Contains(text, `"mcp_servers":{"configured":true,"redacted":true}`) {
		t.Fatalf("sanitized Agent must retain only MCP configured status: %s", text)
	}
}

func TestSanitizeValueRecognizesCamelCaseSecretKeys(t *testing.T) {
	cases := map[string]string{
		"DesktopSessionToken":            "desktop-session-secret",
		"DiscordBotToken":                "discord-bot-secret",
		"TelegramBotToken":               "telegram-bot-secret",
		"ConnectorCredentialsKey":        "connector-credentials-secret",
		"ConnectorCredentialsLegacyKeys": "connector-credentials-legacy-secret",
		"ConnectorGitHubClientSecret":    "github-client-secret",
		"SigningPrivateKey":              "signing-private-secret",
	}
	for key, secret := range cases {
		t.Run(key, func(t *testing.T) {
			payload, err := json.Marshal(sanitizeValue(map[string]any{key: secret}))
			if err != nil {
				t.Fatalf("marshal sanitized %s: %v", key, err)
			}
			text := string(payload)
			if strings.Contains(text, secret) {
				t.Fatalf("sanitized payload leaked CamelCase secret %s: %s", key, text)
			}
			if !strings.Contains(text, `"configured":true`) || !strings.Contains(text, `"redacted":true`) {
				t.Fatalf("sanitized payload lost secret presence for %s: %s", key, text)
			}
		})
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

func TestIntegrityRevisionTracksSecretChangesWithoutExposingThem(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	first, err := integrityRevisionFor(map[string]any{"token": "one", "enabled": true}, key)
	if err != nil {
		t.Fatal(err)
	}
	second, err := integrityRevisionFor(map[string]any{"token": "two", "enabled": true}, key)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("keyed internal revision must detect secret rotation")
	}
	for _, revision := range []string{first, second} {
		if !strings.HasPrefix(revision, "hmac-sha256:") {
			t.Fatalf("unexpected keyed revision = %q", revision)
		}
		if strings.Contains(revision, "one") || strings.Contains(revision, "two") {
			t.Fatalf("keyed revision exposed secret: %q", revision)
		}
	}
}

func TestPlanDigestIsKeyedAndBindsSecretInput(t *testing.T) {
	service := &Service{integrityKey: []byte("0123456789abcdef0123456789abcdef")}
	actor := &resolvedActor{
		Actor: Actor{
			OwnerUserID: "owner", AgentID: "agent", SessionKey: "agent:session",
			RoundID:         "business-round",
			LeaseSessionKey: "runtime:session", LeaseRoundID: "runtime-round",
			SourceContext: "agent",
		},
		Authority: AuthorityAgentSelf,
		Context:   ScopeRef{Kind: ScopeKindAgent, ID: "agent"},
	}
	plan := ChangePlan{
		Domain: DomainProviders, Operation: "update", Target: "provider",
		Scope: ScopeRef{Kind: ScopeKindOwner, ID: "owner"}, CurrentRevision: "revision",
	}
	first, err := service.planDigest(actor, ChangeRequest{
		Domain: plan.Domain, Operation: plan.Operation, Target: plan.Target,
		Input: json.RawMessage(`{"auth_token":"one"}`),
	}, plan)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.planDigest(actor, ChangeRequest{
		Domain: plan.Domain, Operation: plan.Operation, Target: plan.Target,
		Input: json.RawMessage(`{"auth_token":"two"}`),
	}, plan)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || !strings.HasPrefix(first, "hmac-sha256:") {
		t.Fatalf("plan digest did not bind secret input safely: %q %q", first, second)
	}
	changedBusiness := *actor
	changedBusiness.SessionKey = "agent:another-session"
	businessDigest, err := service.planDigest(&changedBusiness, ChangeRequest{
		Domain: plan.Domain, Operation: plan.Operation, Target: plan.Target,
		Input: json.RawMessage(`{"auth_token":"one"}`),
	}, plan)
	if err != nil {
		t.Fatal(err)
	}
	changedLease := *actor
	changedLease.LeaseRoundID = "another-runtime-round"
	leaseDigest, err := service.planDigest(&changedLease, ChangeRequest{
		Domain: plan.Domain, Operation: plan.Operation, Target: plan.Target,
		Input: json.RawMessage(`{"auth_token":"one"}`),
	}, plan)
	if err != nil {
		t.Fatal(err)
	}
	if first == businessDigest || first == leaseDigest {
		t.Fatalf(
			"plan digest did not bind business and lease identity: %q %q %q",
			first,
			businessDigest,
			leaseDigest,
		)
	}
}
