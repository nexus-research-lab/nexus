// INPUT: 当前 runtime Session identity、上下文名称与 computer action。
// OUTPUT: 浏览器状态、标签页、页面/网络数据、交互结果、截图或 PDF MCP content。
// POS: WebBridge 的完整浏览器 computer-use 工具适配层。
package webbridge

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	webbridgesvc "github.com/nexus-research-lab/nexus/internal/service/webbridge"
)

// ServerName 是浏览器控制内建 MCP server 的注册名。
const ServerName = "nexus_browser"

// NewServer 为当前 runtime Session 创建浏览器控制 MCP server。
func NewServer(
	service *webbridgesvc.Service,
	sessionKey string,
	sessionLabel string,
	resolveCDPAccess func(context.Context) (bool, error),
) *sdktool.SimpleSDKMCPServer {
	return sdktool.NewSimpleSDKMCPServer(ServerName, "1.0.0", []sdktool.Tool{{
		Name: "computer",
		Description: "通过 Nexus WebBridge 完整控制 Chromium。" +
			"支持多标签会话、导航与查找、可访问性快照、CSS/ref 交互、键盘鼠标、" +
			"JavaScript、用户启用后的原始 CDP、网络抓取、上传、截图、PDF 和会话关闭。",
		SearchHint:  "browser web computer use navigate evaluate cdp network snapshot click fill screenshot pdf upload tab 浏览器 网页 操作",
		InputSchema: computerSchema(),
		Annotations: &sdktool.ToolAnnotations{
			Destructive: true,
			OpenWorld:   true,
		},
		Handler: func(ctx context.Context, input map[string]any) (sdktool.ToolResult, error) {
			if service == nil {
				return errorResult(errors.New("Nexus WebBridge 未启用")), nil
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
			result, err := service.Execute(ctx, sessionKey, sessionLabel, action, input, allowCDP)
			if err != nil {
				return errorResult(err), nil
			}
			return renderResult(strings.ToLower(strings.TrimSpace(action)), result), nil
		},
	}})
}

func computerSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type": "string",
				"enum": []string{
					"status", "navigate", "find_tab", "evaluate", "network", "snapshot",
					"click", "fill", "mouse_click", "cdp", "key_type", "send_keys",
					"screenshot", "save_as_pdf", "upload", "list_tabs", "close_tab",
					"close_session", "attach_active", "attach_tab", "press_key", "close",
				},
			},
			"url": map[string]any{
				"type": "string", "description": "navigate 的目标 URL，或 find_tab 的 URL/主机名/通配模式。",
			},
			"new_tab": map[string]any{
				"type": "boolean", "description": "navigate 是否新建并保留当前标签页。",
			},
			"active": map[string]any{
				"type": "boolean", "description": "find_tab 是否允许借用当前前台标签页。",
			},
			"tab_id": map[string]any{
				"type": "integer", "minimum": 1, "description": "attach_tab 使用的标签页 ID。",
			},
			"code": map[string]any{
				"type": "string", "description": "evaluate 执行的 JavaScript。",
			},
			"cmd": map[string]any{
				"type": "string", "enum": []string{"start", "stop", "list", "detail"},
				"description": "network 子命令。",
			},
			"filter": map[string]any{
				"type": "string", "description": "network list 的可选文本过滤条件。",
			},
			"request_id": map[string]any{
				"type": "string", "description": "network detail 的请求 ID。",
			},
			"selector": map[string]any{
				"type": "string", "description": "CSS selector 或 snapshot 返回的 @e 引用。",
			},
			"ref": map[string]any{
				"type": "string", "description": "selector 的兼容别名。",
			},
			"value": map[string]any{
				"type": "string", "description": "fill 写入的文本；空字符串表示清空。",
			},
			"method": map[string]any{
				"type": "string", "description": "cdp 调用的方法名。",
			},
			"params": map[string]any{
				"type": "object", "description": "cdp 调用参数。", "additionalProperties": true,
			},
			"text": map[string]any{
				"type": "string", "description": "key_type 插入的任意文本。",
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
			"format": map[string]any{
				"type": "string", "enum": []string{"png", "jpeg"}, "description": "screenshot 图像格式。",
			},
			"quality": map[string]any{
				"type": "integer", "minimum": 0, "maximum": 100, "description": "JPEG 截图质量。",
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
				"type": "string", "description": "PDF 文件名。",
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
		return errorResult(errors.New("WebBridge 二进制结果缺少数据"))
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
				"uri":      "nexus://webbridge/pdf/" + url.PathEscape(fileName),
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
