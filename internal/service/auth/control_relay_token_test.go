package auth

import (
	"context"
	"encoding/json"
	"errors"
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
		UserID:         "owner-local",
		ControlUserID:  "control-user",
		DeploymentID:   "deployment-1",
		OrganizationID: "organization-1",
		AuthMethod:     AuthMethodPassword,
		SessionID:      &sessionID,
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

func TestControlAuthorityVerifiesOrganizationMembers(t *testing.T) {
	const serviceToken = "control-service-token-32-characters"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != controlAPIBase+"/internal/organizations/members/verify" {
			http.NotFound(writer, request)
			return
		}
		var input struct {
			DeploymentID   string   `json:"deployment_id"`
			OrganizationID string   `json:"organization_id"`
			UserIDs        []string `json:"user_ids"`
		}
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Error(err)
			return
		}
		if input.DeploymentID != "deployment-1" || input.OrganizationID != "organization-1" {
			t.Errorf("organization scope = %+v", input)
		}
		if len(input.UserIDs) == 1 && input.UserIDs[0] == "user-other" {
			writer.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(writer).Encode(map[string]any{"code": "operation_forbidden", "message": "forbidden"})
			return
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"code": "0000", "data": map[string]bool{"valid": true}})
	}))
	t.Cleanup(server.Close)
	authority := NewControlAuthority(config.Config{
		ControlURL: server.URL, ControlServiceToken: serviceToken, ControlRequestTimeoutSeconds: 2,
	}, nil, nil)
	sessionID := "session-1"
	principal := &Principal{
		UserID: "owner-local", ControlUserID: "control-user", DeploymentID: "deployment-1",
		OrganizationID: "organization-1", AuthMethod: AuthMethodPassword, SessionID: &sessionID,
	}
	if err := authority.VerifyOrganizationMembers(context.Background(), principal, []string{"user-two"}); err != nil {
		t.Fatal(err)
	}
	if err := authority.VerifyOrganizationMembers(context.Background(), principal, []string{"user-other"}); !errors.Is(err, ErrOrganizationMemberInvalid) {
		t.Fatalf("cross-organization error = %v", err)
	}
}

func TestControlAuthorityVerifiesOwnedAgents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != controlAPIBase+"/internal/agents/verify" {
			http.NotFound(writer, request)
			return
		}
		var input struct {
			DeploymentID   string   `json:"deployment_id"`
			OrganizationID string   `json:"organization_id"`
			OwnerUserID    string   `json:"owner_user_id"`
			AgentIDs       []string `json:"agent_ids"`
		}
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		if input.OwnerUserID != "control-user" || len(input.AgentIDs) != 1 || input.AgentIDs[0] != "agent-1" {
			t.Fatalf("agent verification = %+v", input)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"code": "0000", "data": map[string]any{"agents": []any{}}})
	}))
	t.Cleanup(server.Close)
	authority := NewControlAuthority(config.Config{ControlURL: server.URL, ControlServiceToken: "control-service-token-32-characters", ControlRequestTimeoutSeconds: 2}, nil, nil)
	sessionID := "session-1"
	principal := &Principal{ControlUserID: "control-user", DeploymentID: "deployment-1", OrganizationID: "organization-1", AuthMethod: AuthMethodPassword, SessionID: &sessionID}
	if err := authority.VerifyOwnedAgents(context.Background(), principal, []string{"agent-1"}); err != nil {
		t.Fatal(err)
	}
}
