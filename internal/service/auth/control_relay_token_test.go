package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
)

func TestControlAuthorityExchangesFixedAudienceRelayUserToken(t *testing.T) {
	const serviceToken = "control-service-token-32-characters"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != controlAPIBase+"/internal/humans/verify" {
			http.NotFound(writer, request)
			return
		}
		if request.Header.Get("Authorization") != "Bearer "+serviceToken {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		var input map[string]string
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if input["user_id"] != "control-user" || input["session_id"] != "session-1" {
			t.Errorf("identity = %#v", input)
		}
		if input["audience"] != relayUserPrincipalAudience {
			t.Errorf("audience = %q", input["audience"])
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"code": "0000",
			"data": map[string]string{"principal_token": "relay-principal-token"},
		})
	}))
	t.Cleanup(server.Close)

	authority := NewControlAuthority(config.Config{
		ControlURL:                   server.URL,
		ControlServiceToken:          serviceToken,
		ControlRequestTimeoutSeconds: 2,
	}, nil, nil)
	sessionID := "session-1"
	token, err := authority.ExchangeRelayUserToken(context.Background(), &Principal{
		UserID:        "owner-local",
		ControlUserID: "control-user",
		DeploymentID:  "deployment-1",
		AuthMethod:    AuthMethodPassword,
		SessionID:     &sessionID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if token != "relay-principal-token" {
		t.Fatalf("token = %q", token)
	}
}

func TestControlAuthorityRejectsLocalPrincipalForRelayToken(t *testing.T) {
	authority := NewControlAuthority(config.Config{}, nil, nil)
	if _, err := authority.ExchangeRelayUserToken(context.Background(), &Principal{
		UserID:     "local-owner",
		AuthMethod: AuthMethodLocal,
	}); err == nil {
		t.Fatal("local Principal must not mint a Relay user token")
	}
}
