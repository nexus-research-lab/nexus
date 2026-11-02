// INPUT: 可选 Relay 配置、Team handler 与 Desktop Local authority。
// OUTPUT: Relay 未配置不挂路由、配置后挂路由且 Local authority 不装配 Team 的断言。
// POS: app server 的可选 Team route composition 回归边界。
package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/app"
	"github.com/nexus-research-lab/nexus/internal/config"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	teamhandler "github.com/nexus-research-lab/nexus/internal/handler/team"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	relaysvc "github.com/nexus-research-lab/nexus/internal/service/relay"
)

func TestMountTeamRoutesRequiresRelayURL(t *testing.T) {
	api := handlershared.NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler := teamhandler.New(api, nil, nil)

	disabled := &Server{
		config:   config.Config{APIPrefix: "/nexus/v1"},
		router:   newPathParamRouter(),
		handlers: handlerSet{team: handler},
	}
	disabled.mountTeamRoutes()
	disabledResponse := httptest.NewRecorder()
	disabled.router.ServeHTTP(
		disabledResponse,
		httptest.NewRequest(http.MethodPost, "/nexus/v1/team/rooms", nil),
	)
	if disabledResponse.Code != http.StatusNotFound {
		t.Fatalf("disabled Team route status=%d body=%s", disabledResponse.Code, disabledResponse.Body.String())
	}

	enabled := &Server{
		config: config.Config{
			APIPrefix: "/nexus/v1",
			RelayURL:  "https://relay.example.com",
		},
		router:   newPathParamRouter(),
		handlers: handlerSet{team: handler},
	}
	enabled.mountTeamRoutes()
	enabledResponse := httptest.NewRecorder()
	enabled.router.ServeHTTP(
		enabledResponse,
		httptest.NewRequest(http.MethodPost, "/nexus/v1/team/rooms", nil),
	)
	if enabledResponse.Code != http.StatusForbidden {
		t.Fatalf("enabled Team route status=%d body=%s", enabledResponse.Code, enabledResponse.Body.String())
	}
}

func TestTeamCapabilitiesAndDisabledRoutes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		url      string
		disabled bool
		want     bool
	}{
		{name: "unconfigured"},
		{name: "configured", url: "https://relay.example.com", want: true},
		{name: "explicitly disabled", url: "https://relay.example.com", disabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := handlershared.NewAPI(nil)
			s := &Server{config: config.Config{APIPrefix: "/nexus/v1", RelayURL: tc.url, MultiplayerDisabled: tc.disabled}, router: newPathParamRouter(), api: api, handlers: handlerSet{team: teamhandler.New(api, nil, nil)}}
			s.mountTeamRoutes()
			response := httptest.NewRecorder()
			s.router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/nexus/v1/team/capabilities", nil))
			var body struct {
				Data struct {
					Enabled bool `json:"enabled"`
				} `json:"data"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || response.Code != http.StatusOK || body.Data.Enabled != tc.want {
				t.Fatalf("capabilities: code=%d body=%s err=%v", response.Code, response.Body.String(), err)
			}
			if !tc.want {
				response = httptest.NewRecorder()
				s.router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/nexus/v1/team/invitations", nil))
				if response.Code != http.StatusNotFound {
					t.Fatalf("disabled invitations status=%d", response.Code)
				}
			}
		})
	}
}

func TestNewTeamHandlerRequiresControlAuthority(t *testing.T) {
	api := handlershared.NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil)))
	client, err := relaysvc.NewClient("https://relay.example.com", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	services := &app.AppServices{
		Auth:  authsvc.NewLocalAuthority("", nil, nil),
		Relay: client,
	}
	if handler := newTeamHandler(api, services); handler != nil {
		t.Fatal("Desktop Local authority must not expose Team routes")
	}
}
