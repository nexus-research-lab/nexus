package server

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
)

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
