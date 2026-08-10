package configuration

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

func TestHostDomainSnapshotDoesNotExposeCredentials(t *testing.T) {
	t.Setenv("NEXUS_CONFIG_DIR", t.TempDir())
	secrets := []string{
		"desktop-session-secret",
		"discord-bot-secret",
		"telegram-bot-secret",
		"connector-credentials-secret",
		"github-client-secret",
		"google-client-secret",
		"linkedin-client-secret",
		"twitter-client-secret",
		"instagram-client-secret",
		"shopify-client-secret",
	}
	cfg := config.Config{
		ProjectName:                    "safe-project",
		DesktopSessionToken:            secrets[0],
		DiscordBotToken:                secrets[1],
		TelegramBotToken:               secrets[2],
		ConnectorCredentialsKey:        secrets[3],
		ConnectorGitHubClientID:        "github-client-id",
		ConnectorGitHubClientSecret:    secrets[4],
		ConnectorGoogleClientID:        "google-client-id",
		ConnectorGoogleClientSecret:    secrets[5],
		ConnectorLinkedInClientID:      "linkedin-client-id",
		ConnectorLinkedInClientSecret:  secrets[6],
		ConnectorTwitterClientID:       "twitter-client-id",
		ConnectorTwitterClientSecret:   secrets[7],
		ConnectorInstagramClientID:     "instagram-client-id",
		ConnectorInstagramClientSecret: secrets[8],
		ConnectorShopifyClientID:       "shopify-client-id",
		ConnectorShopifyClientSecret:   secrets[9],
	}
	service := &Service{cfg: cfg}
	actor := &resolvedActor{
		Actor: Actor{
			OwnerUserID:     authctx.SystemUserID,
			AgentID:         "agent-safe",
			IsMainAgent:     true,
			PrincipalRole:   authctx.RoleOwner,
			AuthMethod:      authctx.AuthMethodLocal,
			LocalSingleUser: true,
		},
		Authority: AuthorityOwnerMain,
		Context:   ScopeRef{Kind: ScopeKindOwner, ID: authctx.SystemUserID},
	}

	snapshot, err := service.domainSnapshot(t.Context(), actor, DomainHost, "", false)
	if err != nil {
		t.Fatalf("build host domain snapshot: %v", err)
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal host domain snapshot: %v", err)
	}
	text := string(payload)
	for _, secret := range secrets {
		if strings.Contains(text, secret) {
			t.Fatalf("host domain snapshot leaked %q: %s", secret, text)
		}
	}
	if !strings.Contains(text, "safe-project") {
		t.Fatalf("host domain snapshot lost non-sensitive configuration: %s", text)
	}
	if !strings.Contains(text, `"configured":true`) || !strings.Contains(text, `"redacted":true`) {
		t.Fatalf("host domain snapshot must retain credential presence status: %s", text)
	}
}
