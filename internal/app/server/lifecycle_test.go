package server

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/app"
	"github.com/nexus-research-lab/nexus/internal/config"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
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
