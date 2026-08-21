package browser

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestExecuteKeepsTabOwnershipInsideRuntimeSession(t *testing.T) {
	service := NewService()
	commands := make(chan map[string]any, 8)
	_, detach := service.Attach("0.1.0", "browser-a", "generation-a", func(_ context.Context, payload any) error {
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
		"tab_id": float64(42), "tab_ref": "ref-42", "owned": true, "url": "https://example.com",
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
		"tab_id": float64(43), "tab_ref": "ref-43", "owned": true, "url": "https://example.org",
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
	tabRefs := params["tab_refs"].([]string)
	if len(tabRefs) != 2 || tabRefs[0] != "ref-42" || tabRefs[1] != "ref-43" {
		t.Fatalf("list_tabs 未携带完整会话标签页: %+v", params)
	}
	service.Resolve(command["id"].(string), map[string]any{
		"scope": "session",
		"tabs": []any{
			map[string]any{"tab_id": float64(42), "tab_ref": "ref-42"},
			map[string]any{"tab_id": float64(43), "tab_ref": "ref-43"},
		},
	}, "")
	if err := <-errCh; err != nil {
		t.Fatalf("list_tabs error = %v", err)
	}

	go func() {
		_, err := service.Execute(context.Background(), "session-a", "Agent A", "snapshot", nil, false)
		errCh <- err
	}()
	command = receiveCommand(t, commands)
	params = command["params"].(map[string]any)
	if params["tab_id"] != int64(43) || params["tab_ref"] != "ref-43" {
		t.Fatalf("snapshot 未使用最新 Session tab: %+v", params)
	}
	service.Resolve(command["id"].(string), map[string]any{"snapshot": "", "truncated": false}, "")
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
	_, detach := service.Attach("0.1.0", "browser-a", "generation-a", func(_ context.Context, payload any) error {
		commands <- payload.(map[string]any)
		return nil
	}, nil)
	defer detach()
	service.sessions["session-a"] = browserSession{
		activeTabRef: "ref-42",
		tabs:         map[string]browserTab{"ref-42": {id: 42, ref: "ref-42"}},
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

func TestPrepareParamsCoversBrowserCapabilityInputs(t *testing.T) {
	service := NewService()
	service.sessions["session-a"] = browserSession{
		activeTabRef: "ref-42",
		tabs:         map[string]browserTab{"ref-42": {id: 42, ref: "ref-42"}},
	}

	allTabs, _, err := service.prepareParams("session-a", "Agent A", "list_tabs", map[string]any{
		"scope": "all",
	})
	if err != nil || allTabs["scope"] != "all" {
		t.Fatalf("list_tabs all params = %+v, err = %v", allTabs, err)
	}
	if _, exists := allTabs["tab_refs"]; exists {
		t.Fatalf("list_tabs all 不应限制 Session 标签页: %+v", allTabs)
	}
	attached, _, err := service.prepareParams("session-a", "Agent A", "attach_tab", map[string]any{
		"tab_ref": "ref-99",
	})
	if err != nil || attached["tab_ref"] != "ref-99" {
		t.Fatalf("attach_tab params = %+v, err = %v", attached, err)
	}
	if _, _, err = service.prepareParams("session-a", "Agent A", "attach_tab", map[string]any{
		"tab_id": 99,
	}); err == nil {
		t.Fatal("attach_tab 不应接受可复用的整数 tab_id")
	}

	mouse, _, err := service.prepareParams("session-a", "Agent A", "mouse_click", map[string]any{
		"x": json.Number("12.5"), "y": json.Number("24.5"), "click_count": json.Number("2"),
	})
	if err != nil || mouse["tab_id"] != int64(42) || mouse["tab_ref"] != "ref-42" || mouse["click_count"] != int64(2) {
		t.Fatalf("mouse_click params = %+v, err = %v", mouse, err)
	}

	history, _, err := service.prepareParams("session-a", "Agent A", "history", map[string]any{
		"start_time": json.Number("100"), "end_time": json.Number("200"), "max_results": json.Number("50"),
	})
	if err != nil || history["max_results"] != int64(50) {
		t.Fatalf("history params = %+v, err = %v", history, err)
	}

	for _, test := range []struct {
		name   string
		action string
		input  map[string]any
	}{
		{name: "screenshot quality", action: "screenshot", input: map[string]any{"quality": json.Number("80")}},
		{name: "evaluate timeout", action: "evaluate", input: map[string]any{"code": "Promise.resolve(true)", "timeout_ms": json.Number("30000")}},
		{name: "wait timeout", action: "wait_for_url", input: map[string]any{"url": "*wd=*", "timeout_ms": json.Number("10000")}},
		{name: "console result limit", action: "console", input: map[string]any{"cmd": "list", "max_results": json.Number("10")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := service.prepareParams("session-a", "Agent A", test.action, test.input); err != nil {
				t.Fatalf("%s params error = %v", test.action, err)
			}
		})
	}

	if _, _, err = service.prepareParams("session-a", "Agent A", "drag", map[string]any{
		"from_x": 1, "from_y": 2, "to_x": 3,
	}); err == nil {
		t.Fatal("drag 缺少 to_y 时应拒绝")
	}
}

func TestAttachRejectsStaleBrowserGeneration(t *testing.T) {
	service := NewService()
	_, detach := service.Attach("0.1.0", "browser-a", "generation-a", func(context.Context, any) error {
		return nil
	}, nil)
	defer detach()
	service.sessions["session-a"] = browserSession{
		activeTabRef: "ref-42",
		tabs:         map[string]browserTab{"ref-42": {id: 42, ref: "ref-42"}},
	}

	_, detachNext := service.Attach("0.1.0", "browser-a", "generation-b", func(context.Context, any) error {
		return nil
	}, nil)
	defer detachNext()

	if _, err := service.Execute(context.Background(), "session-a", "Agent A", "snapshot", nil, false); err == nil {
		t.Fatal("扩展代次变化后不应继续使用旧标签页引用")
	}
}

func TestBrowserExtensionHandlesEveryServiceAction(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("无法定位 Browser service 测试文件")
	}
	backgroundPath := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "desktop", "browser-extension", "background.js")
	content, err := os.ReadFile(backgroundPath)
	if err != nil {
		t.Fatalf("读取 Browser 扩展: %v", err)
	}
	if !strings.Contains(string(content), `const PROTOCOL_VERSION = "`+ProtocolVersion+`";`) {
		t.Fatalf("Browser 扩展协议版本未与宿主 %s 对齐", ProtocolVersion)
	}
	for _, action := range SupportedActions() {
		if action == "status" {
			continue
		}
		if !strings.Contains(string(content), `case "`+action+`":`) {
			t.Errorf("Browser 扩展缺少 action %q", action)
		}
	}
}

func receiveCommand(t *testing.T, commands <-chan map[string]any) map[string]any {
	t.Helper()
	select {
	case command := <-commands:
		return command
	case <-time.After(time.Second):
		t.Fatal("未收到 Browser 命令")
		return nil
	}
}
