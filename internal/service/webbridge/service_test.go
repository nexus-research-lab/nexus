package webbridge

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestExecuteKeepsTabOwnershipInsideRuntimeSession(t *testing.T) {
	service := NewService()
	commands := make(chan map[string]any, 8)
	_, detach := service.Attach("0.1.0", func(_ context.Context, payload any) error {
		commands <- payload.(map[string]any)
		return nil
	}, nil)
	defer detach()

	resultCh := make(chan map[string]any, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := service.Execute(context.Background(), "session-a", "Agent A", "navigate", map[string]any{
			"url": "https://example.com",
		}, false)
		resultCh <- result
		errCh <- err
	}()

	command := receiveCommand(t, commands)
	if command["action"] != "navigate" {
		t.Fatalf("首个动作 = %v", command["action"])
	}
	params := command["params"].(map[string]any)
	if _, exists := params["tab_id"]; exists {
		t.Fatalf("首次导航不应携带旧 tab_id: %+v", params)
	}
	if params["session"] != "session-a" || params["group_title"] != "Agent A" {
		t.Fatalf("导航缺少会话分组信息: %+v", params)
	}
	service.Resolve(command["id"].(string), map[string]any{
		"tab_id": float64(42), "owned": true, "url": "https://example.com",
	}, "")
	if err := <-errCh; err != nil {
		t.Fatalf("navigate error = %v", err)
	}
	if result := <-resultCh; result["tab_id"] != float64(42) {
		t.Fatalf("navigate result = %+v", result)
	}

	go func() {
		_, err := service.Execute(context.Background(), "session-a", "Agent A", "navigate", map[string]any{
			"url": "https://example.org", "new_tab": true,
		}, false)
		errCh <- err
	}()
	command = receiveCommand(t, commands)
	params = command["params"].(map[string]any)
	if _, exists := params["tab_id"]; exists {
		t.Fatalf("new_tab 不应复用旧标签页: %+v", params)
	}
	service.Resolve(command["id"].(string), map[string]any{
		"tab_id": float64(43), "owned": true, "url": "https://example.org",
	}, "")
	if err := <-errCh; err != nil {
		t.Fatalf("new tab navigate error = %v", err)
	}

	go func() {
		_, err := service.Execute(context.Background(), "session-a", "Agent A", "list_tabs", nil, false)
		errCh <- err
	}()
	command = receiveCommand(t, commands)
	params = command["params"].(map[string]any)
	tabIDs := params["tab_ids"].([]int64)
	if len(tabIDs) != 2 || tabIDs[0] != 42 || tabIDs[1] != 43 {
		t.Fatalf("list_tabs 未携带完整会话标签页: %+v", params)
	}
	service.Resolve(command["id"].(string), map[string]any{"tabs": []any{}}, "")
	if err := <-errCh; err != nil {
		t.Fatalf("list_tabs error = %v", err)
	}

	go func() {
		_, err := service.Execute(context.Background(), "session-a", "Agent A", "snapshot", nil, false)
		errCh <- err
	}()
	command = receiveCommand(t, commands)
	params = command["params"].(map[string]any)
	if params["tab_id"] != int64(43) {
		t.Fatalf("snapshot 未使用最新 Session tab: %+v", params)
	}
	service.Resolve(command["id"].(string), map[string]any{"tree": []any{}}, "")
	if err := <-errCh; err != nil {
		t.Fatalf("snapshot error = %v", err)
	}

	if _, err := service.Execute(context.Background(), "session-b", "Agent B", "snapshot", nil, false); err == nil {
		t.Fatal("另一 Session 不应继承 session-a 的标签页")
	}
}

func TestExecuteBlocksRawCDPByDefault(t *testing.T) {
	service := NewService()
	if _, err := service.Execute(
		context.Background(),
		"session-a",
		"Agent A",
		"cdp",
		map[string]any{"method": "Browser.getVersion"},
		false,
	); !errors.Is(err, ErrCDPDisabled) {
		t.Fatalf("cdp error = %v", err)
	}

	commands := make(chan map[string]any, 1)
	_, detach := service.Attach("0.1.0", func(_ context.Context, payload any) error {
		commands <- payload.(map[string]any)
		return nil
	}, nil)
	defer detach()
	service.sessions["session-a"] = browserSession{
		activeTabID: 42,
		tabIDs:      map[int64]struct{}{42: {}},
	}
	errCh := make(chan error, 1)
	go func() {
		_, err := service.Execute(
			context.Background(),
			"session-a",
			"Agent A",
			"cdp",
			map[string]any{"method": "Browser.getVersion"},
			true,
		)
		errCh <- err
	}()
	command := receiveCommand(t, commands)
	service.Resolve(command["id"].(string), map[string]any{"product": "Chrome"}, "")
	if err := <-errCh; err != nil {
		t.Fatalf("启用后的 cdp error = %v", err)
	}
}

func receiveCommand(t *testing.T, commands <-chan map[string]any) map[string]any {
	t.Helper()
	select {
	case command := <-commands:
		return command
	case <-time.After(time.Second):
		t.Fatal("未收到 WebBridge 命令")
		return nil
	}
}
