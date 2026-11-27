// routes_and_lifecycle_test.go 覆盖 HTTP 服务生命周期、路由、代理、目录监听与冒烟测试。
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"
	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus/internal/app"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	teamhandler "github.com/nexus-research-lab/nexus/internal/handler/team"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
	launchersvc "github.com/nexus-research-lab/nexus/internal/service/launcher"
	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
	relaysvc "github.com/nexus-research-lab/nexus/internal/service/relay"
	"github.com/nexus-research-lab/nexus/internal/storage"
	"github.com/pressly/goose/v3"
)

func TestStartConfigurationRecoveryRunsInitialSweep(t *testing.T) {
	cfg := config.Config{
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(t.TempDir(), "nexus.db"),
	}
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.Up(db, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(
		t.Context(),
		`INSERT INTO configuration_changes (
			request_id, owner_user_id, actor_agent_id, domain, operation, status, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, datetime('now', '-10 minutes'))`,
		"request-startup-recovery", "owner-startup", "nexus", "preferences", "update", "applying",
	); err != nil {
		t.Fatal(err)
	}

	server := &Server{
		api: handlershared.NewAPI(logx.NewDiscardLogger()),
		services: &app.AppServices{
			Configuration: configurationsvc.NewService(cfg, db, nil, nil, nil, nil, nil, nil, nil),
		},
	}
	stop, err := server.startConfigurationRecovery(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stop == nil {
		t.Fatal("configuration recovery returned nil stop")
	}
	stop()

	var status, result string
	if err := db.QueryRowContext(
		t.Context(),
		`SELECT status, result_json FROM configuration_changes WHERE owner_user_id = ?`, "owner-startup",
	).Scan(&status, &result); err != nil {
		t.Fatal(err)
	}
	if status != "reconcile_required" || !strings.Contains(result, `"applied":"unknown"`) {
		t.Fatalf("startup recovery status=%q result=%s", status, result)
	}
}

func TestServerCloseWaitsForHTTPDrain(t *testing.T) {
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := reservation.Addr().String()
	port := reservation.Addr().(*net.TCPAddr).Port
	reservation.Close()
	entered, release := make(chan struct{}), make(chan struct{})
	s := &Server{config: config.Config{Host: "127.0.0.1", Port: port}, router: newPathParamRouter(), api: handlershared.NewAPI(logx.NewDiscardLogger())}
	s.router.Get("/", func(w http.ResponseWriter, r *http.Request) { close(entered); <-release; io.WriteString(w, "drained") })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer s.Close(context.Background())
	// 失败路径也释放 handler，避免测试清理阻塞。
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	served := make(chan error, 1)
	go func() { served <- s.ListenAndServe(ctx) }()
	response := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 4 * time.Second}
		defer client.CloseIdleConnections()
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			result, err := client.Get("http://" + address + "/")
			if err == nil {
				body, readErr := io.ReadAll(result.Body)
				result.Body.Close()
				if readErr == nil && string(body) != "drained" {
					readErr = errors.New("响应未排空")
				}
				response <- readErr
				return
			}
			select {
			case <-ctx.Done():
				response <- err
				return
			case <-ticker.C:
			}
		}
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("HTTP 未开始处理请求")
	}
	short, stop := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer stop()
	if err := s.Close(short); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Close=%v, want deadline", err)
	}
	select {
	case err := <-served:
		t.Fatalf("排空前返回: %v", err)
	default:
	}
	close(release)
	if err := s.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-served; !errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("Serve=%v", err)
	}
	if err := <-response; err != nil {
		t.Fatal(err)
	}
	if err := s.ListenAndServe(ctx); err == nil {
		t.Fatal("已关闭服务不能重新启动")
	}
}

func TestServerListenFailureReleasesLifecycle(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	s := &Server{config: config.Config{Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port}}
	if err := s.ListenAndServe(context.Background()); err == nil {
		t.Fatal("端口冲突应启动失败")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if s.done != nil || s.cancel != nil {
		t.Fatal("失败启动遗留运行状态")
	}
}

type browserSmokeFactory struct{ binary string }

func (f browserSmokeFactory) New(options bridge.Options) runtimectx.Client {
	options.CLIPath = f.binary
	return runtimectx.NewAgentClient(options)
}

func TestBrowserSmoke(t *testing.T) {
	if os.Getenv("NEXUS_BROWSER_SMOKE") != "1" {
		t.Skip("set NEXUS_BROWSER_SMOKE=1 after building web and installing Chromium")
	}
	binary := os.Getenv("NEXUS_SMOKE_NXS_BINARY")
	if binary == "" {
		t.Fatal("NEXUS_SMOKE_NXS_BINARY must point to the paired nxs runtime")
	}
	if !filepath.IsAbs(binary) {
		t.Fatal("runtime path must be absolute")
	}
	webRoot, err := filepath.Abs("../../../web")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(webRoot, "dist", "app.html")); err != nil {
		t.Fatal("build web before smoke: ", err)
	}
	pnpm, err := exec.LookPath("pnpm")
	if err != nil {
		t.Fatal(err)
	}
	browserCache := os.Getenv("PLAYWRIGHT_BROWSERS_PATH")
	if browserCache == "" {
		cacheRoot, err := os.UserCacheDir()
		if err != nil {
			t.Fatal(err)
		}
		browserCache = filepath.Join(cacheRoot, "ms-playwright")
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, event := range []map[string]any{
			{"type": "message_start", "message": map[string]any{"id": "smoke-reply", "model": "claude-smoke", "role": "assistant", "content": []any{}, "usage": map[string]int{"input_tokens": 1}}},
			{"type": "content_block_start", "index": 0, "content_block": map[string]string{"type": "text", "text": ""}},
			{"type": "content_block_delta", "index": 0, "delta": map[string]string{"type": "text_delta", "text": "Nexus real backend smoke reply"}},
			{"type": "content_block_stop", "index": 0},
			{"type": "message_delta", "delta": map[string]string{"stop_reason": "end_turn"}, "usage": map[string]int{"output_tokens": 8}},
			{"type": "message_stop"},
		} {
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event["type"], data)
		}
	}))
	defer provider.Close()
	cfg := handlertest.NewConfig(t)
	cfg.AppMode = "desktop"
	cfg.DesktopSessionToken = "smoke-local-token"
	cfg.WebDistDir = filepath.Join(webRoot, "dist")
	cfg.MultiplayerDisabled = true
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	server, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	handlertest.CloseServer(t, server)
	ctx := authctx.WithPrincipal(t.Context(), &authctx.Principal{UserID: authctx.SystemUserID, Username: "local", Role: "admin", AuthMethod: "local"})
	record, err := server.services.Provider.Create(ctx, providersvc.CreateInput{Provider: "smoke-provider", PresetKey: "custom", APIFormat: "anthropic_messages", AuthToken: "local-only", BaseURL: provider.URL, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	yes := true
	_, err = server.services.Provider.UpdateModel(ctx, record.Provider, "claude-smoke", providersvc.UpdateModelInput{Enabled: true, IsDefault: true, CapabilitiesOverride: providersvc.ModelCapabilities{TextOutput: &yes, ToolCalling: &yes}})
	if err != nil {
		t.Fatal(err)
	}
	manager := runtimectx.NewManagerWithFactory(browserSmokeFactory{binary: binary})
	defer manager.Close(context.Background())
	server.services.DM.Runtime = manager
	server.services.RoomRealtime.Runtime = manager
	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()
	command := exec.CommandContext(t.Context(), pnpm, "exec", "playwright", "test", "--config", "playwright.smoke.config.ts")
	command.Dir = webRoot
	command.Env = append(os.Environ(), "NEXUS_SMOKE_BASE_URL="+httpServer.URL, "NEXUS_SMOKE_TOKEN="+cfg.DesktopSessionToken, "PLAYWRIGHT_BROWSERS_PATH="+browserCache)
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
}

func TestDesktopTeamCapabilityIsolation(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		t.Run(strconv.FormatBool(disabled), func(t *testing.T) {
			calls := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":{"enabled":false},"success":true,"code":"0000"}`))
			}))
			defer upstream.Close()
			s := &Server{config: config.Config{AppMode: "desktop", APIPrefix: "/nexus/v1", RemoteURL: upstream.URL, MultiplayerDisabled: disabled}, api: handlershared.NewAPI(nil), router: newPathParamRouter()}
			s.mountRemoteGateway()
			s.mountTeamRoutes()
			response := httptest.NewRecorder()
			s.router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/nexus/v1/team/capabilities", nil))
			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"enabled":false`) {
				t.Fatalf("capability status=%d body=%s", response.Code, response.Body.String())
			}
			if (calls == 0) != disabled {
				t.Fatalf("upstream calls=%d disabled=%v", calls, disabled)
			}
		})
	}
}

func TestDesktopRemoteGatewayKeepsSessionOnLocalOrigin(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Origin") != "http://"+request.Host ||
			request.Header.Get(handlershared.DesktopSessionTokenHeader) != "" {
			http.Error(writer, "bad forwarded request", http.StatusBadRequest)
			return
		}
		if _, err := request.Cookie(handlershared.DesktopSessionTokenCookie); err == nil {
			http.Error(writer, "desktop cookie leaked", http.StatusBadRequest)
			return
		}
		switch request.URL.Path {
		case "/auth/v1/login":
			http.SetCookie(writer, &http.Cookie{
				Name: "nexus_session", Value: "opaque", Path: "/", Secure: true, HttpOnly: true,
			})
			writer.WriteHeader(http.StatusOK)
		case "/nexus/v1/team/rooms":
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(upstream.Close)
	server := &Server{
		config: config.Config{
			AppMode: "desktop", APIPrefix: "/nexus/v1", RemoteURL: upstream.URL,
			AuthSessionCookieName: "nexus_session",
			DesktopSessionToken:   "desktop-token",
		},
		api:    handlershared.NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil))),
		router: newPathParamRouter(),
	}
	server.mountRemoteGateway()

	request := httptest.NewRequest(http.MethodPost, "http://app.local/auth/v1/login", nil)
	request.Header.Set("Origin", "http://app.local")
	request.AddCookie(&http.Cookie{Name: handlershared.DesktopSessionTokenCookie, Value: "desktop-token"})
	response := httptest.NewRecorder()
	server.router.ServeHTTP(response, request)
	cookies := response.Result().Cookies()
	if response.Code != http.StatusOK || len(cookies) != 1 || cookies[0].Secure || !cookies[0].HttpOnly {
		t.Fatalf("status = %d, cookies = %+v", response.Code, cookies)
	}

	request = httptest.NewRequest(http.MethodPost, "http://app.local/auth/v1/login", nil)
	request.Header.Set("Origin", "https://attacker.example")
	request.AddCookie(&http.Cookie{Name: handlershared.DesktopSessionTokenCookie, Value: "desktop-token"})
	response = httptest.NewRecorder()
	server.router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "http://app.local/nexus/v1/team/rooms", nil)
	request.Header.Set("Origin", "http://app.local")
	request.Header.Set(handlershared.DesktopSessionTokenHeader, "desktop-token")
	response = httptest.NewRecorder()
	server.router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("Team proxy status = %d", response.Code)
	}
}

func TestExternalRoomChangeReachesDirectorySubscriber(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	server, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	handlertest.CloseServer(t, server)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stop, err := server.startDirectoryWatch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()
	connection, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(httpServer.URL, "http")+cfg.APIPrefix+"/chat/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.CloseNow()
	if err := wsjson.Write(ctx, connection, map[string]any{"type": "subscribe_app_events"}); err != nil {
		t.Fatal(err)
	}
	var subscribed protocol.EventMessage
	if err := wsjson.Read(ctx, connection, &subscribed); err != nil {
		t.Fatal(err)
	}
	if subscribed.EventType != protocol.EventTypeDirectoryChanged || subscribed.Data["directory_revision"] == nil {
		t.Fatalf("missing versioned subscription reconciliation: %+v", subscribed)
	}
	// 与 nexusctl 相同的服务装配，使用独立 DB 连接且不注入 HTTP notifier。
	agents, externalDB, err := app.NewAgentService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer externalDB.Close()
	rooms := app.NewRoomServiceWithDB(cfg, externalDB, agents)
	member, err := agents.CreateAgent(ctx, protocol.CreateRequest{Name: "Directory member"})
	if err != nil {
		t.Fatal(err)
	}
	created, err := rooms.CreateRoom(ctx, protocol.CreateRoomRequest{Name: "External group", AgentIDs: []string{member.AgentID}})
	if err != nil {
		t.Fatal(err)
	}
	awaitName := func(name string) {
		t.Helper()
		for {
			var event protocol.EventMessage
			if err := wsjson.Read(ctx, connection, &event); err != nil {
				t.Fatal(err)
			}
			if event.EventType != protocol.EventTypeDirectoryChanged {
				continue
			}
			recorder := httptest.NewRecorder()
			server.Router().ServeHTTP(recorder, httptest.NewRequest("GET", cfg.APIPrefix+"/launcher/bootstrap", nil))
			var payload struct {
				Data launchersvc.BootstrapResponse `json:"data"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if recorder.Code != 200 {
				t.Fatalf("bootstrap status = %d", recorder.Code)
			}
			for _, room := range payload.Data.Rooms {
				if room.ID != created.Room.ID || room.Name != name {
					continue
				}
				for _, conversation := range payload.Data.Conversations {
					if conversation.RoomID == room.ID && conversation.ConversationID == created.Conversation.ID {
						return
					}
				}
				t.Fatal("new Room is missing the conversation required by the sidebar")
			}
		}
	}
	awaitName("External group")
	renamed := "Renamed external group"
	if _, err := rooms.UpdateRoom(ctx, created.Room.ID, protocol.UpdateRoomRequest{Name: &renamed}); err != nil {
		t.Fatal(err)
	}
	awaitName(renamed)

	// 宿主自身 mutation 已即时通知时，观察器不得在下一轮再补发一次。
	inProcessName := "In-process group"
	if _, err := server.services.Core.Room.UpdateRoom(ctx, created.Room.ID, protocol.UpdateRoomRequest{Name: &inProcessName}); err != nil {
		t.Fatal(err)
	}
	server.handlers.websocket.BroadcastDirectoryChanged(ctx, "room_updated", map[string]any{"room_id": created.Room.ID})
	// 观察器恰在提交与覆盖记录之间轮询时允许先补发一次；即时通知之后不得再重复。
	for {
		var immediate protocol.EventMessage
		if err := wsjson.Read(ctx, connection, &immediate); err != nil {
			t.Fatal(err)
		}
		if immediate.EventType != protocol.EventTypeDirectoryChanged {
			continue
		}
		if immediate.Data["reason"] == "room_updated" {
			break
		}
		if immediate.Data["reason"] != "directory_revision_changed" {
			t.Fatalf("unexpected directory notification = %+v", immediate)
		}
	}
	quietCtx, stopQuiet := context.WithTimeout(ctx, 2500*time.Millisecond)
	defer stopQuiet()
	var duplicate protocol.EventMessage
	if err := wsjson.Read(quietCtx, connection, &duplicate); err == nil {
		t.Fatalf("watcher repeated a covered in-process change: %+v", duplicate)
	}
}

func TestPathParamRouterPreservesProviderModelIDForLegacyServiceDecoder(t *testing.T) {
	t.Parallel()

	router := newPathParamRouter()
	router.Get("/models/{model_id}", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.WriteString(
			writer,
			chi.URLParam(request, "model_id")+"|"+handlershared.PathParam(request, "model_id"),
		)
	})
	server := httptest.NewServer(router)
	defer server.Close()

	response, err := server.Client().Get(server.URL + "/models/namespace%2Fmodel")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}
	if got := strings.TrimSpace(string(payload)); got != "namespace%2Fmodel|namespace%2Fmodel" {
		t.Fatalf("model_id = %q, want service-compatible escaped value", got)
	}
}

func TestChannelCurrentLoginGETAndStartPOSTRemainSeparateRoutes(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	server, err := New(cfg)
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	handlertest.CloseServer(t, server)

	path := cfg.APIPrefix + "/capability/channels/feishu/login"
	getRecorder := httptest.NewRecorder()
	server.Router().ServeHTTP(
		getRecorder,
		httptest.NewRequest(http.MethodGet, path, nil),
	)
	assertChannelRouteFailure(
		t,
		getRecorder,
		http.StatusNotFound,
		"channel.read_login_not_found",
		protocol.FailureEffectNotApplicable,
	)

	postRecorder := httptest.NewRecorder()
	server.Router().ServeHTTP(
		postRecorder,
		httptest.NewRequest(http.MethodPost, path, nil),
	)
	assertChannelRouteFailure(
		t,
		postRecorder,
		http.StatusConflict,
		"channel.start_login_config_required",
		protocol.FailureEffectNotApplied,
	)
}

func assertChannelRouteFailure(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	wantStatus int,
	wantCode string,
	wantEffect protocol.FailureEffect,
) {
	t.Helper()
	if recorder.Code != wantStatus {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, wantStatus, recorder.Body.String())
	}
	var payload struct {
		Data struct {
			Failure protocol.FailureCore `json:"failure"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v body=%s", err, recorder.Body.String())
	}
	if payload.Data.Failure.Code != wantCode || payload.Data.Failure.Effect != wantEffect {
		t.Fatalf("failure = %+v", payload.Data.Failure)
	}
}

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
