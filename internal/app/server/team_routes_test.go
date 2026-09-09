// INPUT: 可选 Relay 配置、Team handler 与 Desktop Local authority。
// OUTPUT: Relay 未配置不挂路由、配置后挂路由且 Local authority 不装配 Team 的断言。
// POS: app server 的可选 Team route composition 回归边界。
package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	teamhandler "github.com/nexus-research-lab/nexus/internal/handler/team"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	relaysvc "github.com/nexus-research-lab/nexus/internal/service/relay"
)

func TestMountTeamRoutesRequiresRelayURL(t *testing.T) {
	api := handlershared.NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler := teamhandler.New(api, nil, nil, nil)

	disabled := &Server{
		config:   config.Config{APIPrefix: "/nexus/v1"},
		router:   newPathParamRouter(),
		handlers: handlerSet{team: handler},
	}
	disabled.mountTeamRoutes()
	disabledResponse := httptest.NewRecorder()
	disabled.router.ServeHTTP(
		disabledResponse,
		httptest.NewRequest(http.MethodPost, "/nexus/v1/team/bootstrap", nil),
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
		httptest.NewRequest(http.MethodPost, "/nexus/v1/team/bootstrap", nil),
	)
	if enabledResponse.Code != http.StatusForbidden {
		t.Fatalf("enabled Team route status=%d body=%s", enabledResponse.Code, enabledResponse.Body.String())
	}
}

func TestNewTeamHandlerRequiresControlAuthority(t *testing.T) {
	api := handlershared.NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil)))
	client, err := relaysvc.NewClient("https://relay.example.com", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	services := &AppServices{
		Auth:  authsvc.NewLocalAuthority("", nil, nil),
		Relay: client,
	}
	if handler := newTeamHandler(api, services); handler != nil {
		t.Fatal("Desktop Local authority must not expose Team routes")
	}
}
