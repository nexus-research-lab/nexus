// INPUT: 当前 runtime Session/round identity、上下文名称与 browser action。
// OUTPUT: 浏览器状态、标签页、页面/网络数据、交互结果、截图或 PDF MCP content。
// POS: Browser 的完整浏览器工具适配层。
package browser

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	browsersvc "github.com/nexus-research-lab/nexus/internal/service/browser"
)

// ServerName 是浏览器控制内建 MCP server 的注册名。
const ServerName = "nexus_browser"

// NewServer 为当前 runtime Session 创建浏览器控制 MCP server。
func NewServer(
	service *browsersvc.Service,
	sessionKey string,
	roundID string,
	sessionLabel string,
	resolveCDPAccess func(context.Context) (bool, error),
) *sdktool.SimpleSDKMCPServer {
	return sdktool.NewSimpleSDKMCPServer(ServerName, "1.0.0", []sdktool.Tool{{
		Name: "browser",
		Description: "通过 Nexus Browser 完整控制 Chromium。" +
			"支持标签页、历史、导航、页面内容、可访问性快照、CSS/ref 与坐标交互、" +
			"JavaScript、CDP、网络、控制台、对话框、剪贴板、上传下载、截图与 PDF。" +
			"snapshot 默认返回相对上一版的增量；操作后再取新快照，不要在无变化时重复调用。",
		SearchHint:  "browser chrome web navigate history cdp network snapshot click fill screenshot download tab 浏览器 网页 操作",
		InputSchema: browserSchema(),
		Annotations: &sdktool.ToolAnnotations{
			Destructive: true,
			OpenWorld:   true,
		},
		Handler: func(ctx context.Context, input map[string]any) (sdktool.ToolResult, error) {
			if service == nil {
				return errorResult(errors.New("Nexus Browser 未启用")), nil
			}
			action, _ := input["action"].(string)
			allowCDP := false
			if strings.EqualFold(strings.TrimSpace(action), "cdp") && resolveCDPAccess != nil {
				var err error
				allowCDP, err = resolveCDPAccess(ctx)
				if err != nil {
					return errorResult(err), nil
				}
			}
			result, err := service.Execute(ctx, sessionKey, roundID, sessionLabel, action, input, allowCDP)
			if err != nil {
				return errorResult(err), nil
			}
			return renderResult(strings.ToLower(strings.TrimSpace(action)), result), nil
		},
	}})
}

func browserSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        browsersvc.SupportedActions(),
				"description": "一次只执行一个 action。页面交互优先先用 snapshot 获取最新 @e ref，再调用固定动作；固定动作无法完成时才使用 evaluate 或 cdp。",
			},
			"url": map[string]any{
				"type": "string", "description": "navigate/download 的目标 URL，或 find_tab/wait_for_url 的 URL、主机名或通配模式。",
			},
			"new_tab": map[string]any{
				"type": "boolean", "description": "navigate 是否新建并保留当前标签页。",
			},
			"active": map[string]any{
				"type": "boolean", "description": "find_tab 是否允许借用当前前台标签页。",
			},
			"tab_ref": map[string]any{
				"type": "string", "description": "attach_tab 使用的不透明标签页引用；只能取自最近一次 list_tabs 结果。",
			},
			"mark": map[string]any{
				"type": "string", "enum": []string{"none", "deliverable", "handoff"},
				"description": "mark_tab 设置本轮结束策略：deliverable 保留页面并交还用户，handoff 留给下一轮继续控制，none 恢复默认。",
			},
			"scope": map[string]any{
				"type": "string", "enum": []string{"session", "all"},
				"description": "list_tabs 列出当前 Session 或全部浏览器标签页。",
			},
			"query": map[string]any{
				"type": "string", "description": "history/downloads 的可选搜索文本。",
			},
			"start_time": map[string]any{
				"type": "number", "minimum": 0, "description": "history 起始 Unix 毫秒时间戳。",
			},
			"end_time": map[string]any{
				"type": "number", "minimum": 0, "description": "history 结束 Unix 毫秒时间戳。",
			},
			"max_results": map[string]any{
				"type": "integer", "minimum": 1, "maximum": 1000,
				"description": "history、console 或 downloads 最大返回数量。",
			},
			"code": map[string]any{
				"type": "string", "description": "evaluate 执行的一段有效 JavaScript；返回 Promise 时会等待其完成。",
			},
			"cmd": map[string]any{
				"type":        "string",
				"enum":        []string{"start", "stop", "list", "detail", "get", "accept", "dismiss", "wait", "show", "read", "write"},
				"description": "network、console、dialog、downloads 或 clipboard 子命令。",
			},
			"filter": map[string]any{
				"type": "string", "description": "network/console list 的可选文本过滤条件。",
			},
			"request_id": map[string]any{
				"type": "string", "description": "network detail 的请求 ID。",
			},
			"selector": map[string]any{
				"type": "string", "description": "CSS selector 或 snapshot 返回的 @e 引用。多数 DOM/鼠标动作使用。",
			},
			"ref": map[string]any{
				"type": "string", "description": "selector 的兼容别名。",
			},
			"target_selector": map[string]any{
				"type": "string", "description": "drag 的目标 CSS selector 或 @e 引用。",
			},
			"value": map[string]any{
				"type": "string", "description": "fill/select_option 写入的值；空字符串可用于清空。",
			},
			"values": map[string]any{
				"type": "array", "items": map[string]any{"type": "string"},
				"description": "select_option 选择的一个或多个值。",
			},
			"state": map[string]any{
				"type": "string", "enum": []string{"attached", "detached", "visible", "hidden"},
				"description": "wait_for 等待的元素状态。",
			},
			"timeout_ms": map[string]any{
				"type": "integer", "minimum": 100, "maximum": 80000,
				"description": "evaluate、wait_for、wait_for_url 或 downloads wait 的等待时长。",
			},
			"page_format": map[string]any{
				"type": "string", "enum": []string{"text", "html"},
				"description": "page_content 返回纯文本或 HTML。",
			},
			"max_chars": map[string]any{
				"type": "integer", "minimum": 1, "maximum": 2000000,
				"description": "page_content 最大字符数。",
			},
			"method": map[string]any{
				"type": "string", "description": "cdp 调用的方法名。",
			},
			"params": map[string]any{
				"type": "object", "description": "cdp 调用参数。", "additionalProperties": true,
			},
			"text": map[string]any{
				"type": "string", "description": "key_type、wait_for 或 clipboard write 使用的文本。",
			},
			"keys": map[string]any{
				"type": "string", "description": "send_keys 的按键序列，例如 Mod+A Enter 或 Shift+Tab。",
			},
			"key": map[string]any{
				"type": "string", "description": "press_key 的兼容单键参数。",
			},
			"repeat": map[string]any{
				"type": "integer", "minimum": 1, "maximum": 100, "description": "send_keys 重复次数。",
			},
			"x":       map[string]any{"type": "number", "description": "鼠标动作的视口 X 坐标。"},
			"y":       map[string]any{"type": "number", "description": "鼠标动作的视口 Y 坐标。"},
			"from_x":  map[string]any{"type": "number", "description": "drag 起点 X 坐标。"},
			"from_y":  map[string]any{"type": "number", "description": "drag 起点 Y 坐标。"},
			"to_x":    map[string]any{"type": "number", "description": "drag 终点 X 坐标。"},
			"to_y":    map[string]any{"type": "number", "description": "drag 终点 Y 坐标。"},
			"delta_x": map[string]any{"type": "number", "description": "scroll 横向滚动量。"},
			"delta_y": map[string]any{"type": "number", "description": "scroll 纵向滚动量。"},
			"steps": map[string]any{
				"type": "integer", "minimum": 1, "maximum": 100, "description": "drag 插值步数。",
			},
			"button": map[string]any{
				"type": "string", "enum": []string{"left", "middle", "right", "back", "forward"},
				"description": "mouse_click/double_click 使用的鼠标按钮。",
			},
			"click_count": map[string]any{
				"type": "integer", "minimum": 1, "maximum": 3, "description": "mouse_click 点击次数。",
			},
			"format": map[string]any{
				"type": "string", "enum": []string{"png", "jpeg"}, "description": "screenshot 图像格式。",
			},
			"quality": map[string]any{
				"type": "integer", "minimum": 0, "maximum": 100, "description": "JPEG 截图质量。",
			},
			"full_page": map[string]any{
				"type": "boolean", "description": "screenshot 是否捕获完整页面。",
			},
			"full": map[string]any{
				"type": "boolean", "description": "snapshot 是否强制返回完整 AX 树；默认在合适时返回相对上一版的增量。",
			},
			"paper_format": map[string]any{
				"type": "string", "enum": []string{"letter", "legal", "a4", "a3", "tabloid"},
				"description": "save_as_pdf 纸张规格。",
			},
			"scale": map[string]any{
				"type": "number", "minimum": 0.1, "maximum": 2, "description": "PDF 缩放比例。",
			},
			"landscape": map[string]any{
				"type": "boolean", "description": "PDF 是否横向。",
			},
			"print_background": map[string]any{
				"type": "boolean", "description": "PDF 是否打印页面背景。",
			},
			"file_name": map[string]any{
				"type": "string", "description": "PDF 或 download 文件名。",
			},
			"save_as": map[string]any{
				"type": "boolean", "description": "download 是否弹出另存为窗口。",
			},
			"download_id": map[string]any{
				"type": "integer", "minimum": 1, "description": "downloads wait/show 使用的下载 ID。",
			},
			"download_state": map[string]any{
				"type": "string", "enum": []string{"in_progress", "complete", "interrupted"},
				"description": "downloads list 的状态过滤。",
			},
			"prompt_text": map[string]any{
				"type": "string", "description": "dialog accept 提交给 prompt 的文本。",
			},
			"files": map[string]any{
				"type": "array", "minItems": 1, "items": map[string]any{"type": "string"},
				"description": "upload 写入 file input 的本机绝对路径。",
			},
		},
		"required":             []string{"action"},
		"additionalProperties": false,
	}
}

func renderResult(action string, result map[string]any) sdktool.ToolResult {
	switch action {
	case "screenshot":
		return binaryResult(result, "image")
	case "save_as_pdf":
		return binaryResult(result, "resource")
	default:
		return jsonResult(result)
	}
}

func binaryResult(result map[string]any, contentType string) sdktool.ToolResult {
	data, _ := result["data"].(string)
	mimeType, _ := result["mime_type"].(string)
	if strings.TrimSpace(data) == "" || strings.TrimSpace(mimeType) == "" {
		return errorResult(errors.New("Browser 二进制结果缺少数据"))
	}
	metadata := make(map[string]any, len(result)-1)
	for key, value := range result {
		if key != "data" {
			metadata[key] = value
		}
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		return errorResult(err)
	}
	content := map[string]any{"type": "image", "data": data, "mimeType": mimeType}
	if contentType == "resource" {
		fileName, _ := result["file_name"].(string)
		content = map[string]any{
			"type": "resource",
			"resource": map[string]any{
				"uri":      "nexus://browser/pdf/" + url.PathEscape(fileName),
				"mimeType": mimeType,
				"blob":     data,
			},
		}
	}
	return sdktool.ToolResult{
		Content:           []map[string]any{content, {"type": "text", "text": string(payload)}},
		StructuredContent: metadata,
	}
}

func jsonResult(result map[string]any) sdktool.ToolResult {
	payload, err := json.Marshal(result)
	if err != nil {
		return errorResult(err)
	}
	return sdktool.ToolResult{
		Content:           []map[string]any{{"type": "text", "text": string(payload)}},
		StructuredContent: result,
	}
}

func errorResult(err error) sdktool.ToolResult {
	return sdktool.ToolResult{
		Content: []map[string]any{{"type": "text", "text": err.Error()}},
		IsError: true,
	}
}
