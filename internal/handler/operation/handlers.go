// INPUT: 操作舞台快照、在线租约与 Navi 公网页面请求。
// OUTPUT: 舞台状态 JSON 或供 sandbox iframe 使用的 HTML 页面快照。
// POS: operation service 的 HTTP 入口；不承载桌面投影和页面抓取规则。
package operation

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	operationpkg "github.com/nexus-research-lab/nexus/internal/service/operation"
)

// Handlers 封装操作舞台 HTTP handlers。
type Handlers struct {
	api       *handlershared.API
	operation *operationpkg.Service
}

// New 创建操作舞台 handlers。
func New(api *handlershared.API, operation *operationpkg.Service) *Handlers {
	return &Handlers{
		api:       api,
		operation: operation,
	}
}

type saveStageSnapshotRequest struct {
	Key      string          `json:"key"`
	Snapshot json.RawMessage `json:"snapshot"`
}

type stagePresenceRequest struct {
	SessionKey string `json:"session_key"`
	ClientID   string `json:"client_id"`
}

var browserPageErrorTemplate = template.Must(template.New("browser-page-error").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <style>
    :root { color-scheme: light; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
    * { box-sizing: border-box; }
    body { margin: 0; min-height: 100vh; display: grid; place-items: center; background: #f7f8fa; color: #242b36; }
    main { width: min(520px, calc(100vw - 48px)); padding: 28px; border: 1px solid #e2e6eb; border-radius: 12px; background: #fff; }
    h1 { margin: 0; font-size: 20px; font-weight: 650; letter-spacing: 0; }
    p { margin: 12px 0 0; color: #697382; font-size: 13px; line-height: 1.7; overflow-wrap: anywhere; }
  </style>
</head>
<body>
  <main><h1>{{.Title}}</h1><p>{{.Detail}}</p></main>
  <script>parent.postMessage({ source: "nexus-navi-proxy", type: "load-error", status: {{.Status}} }, "*");</script>
</body>
</html>`))

// HandleGetStageSnapshot 读取会话舞台快照。
func (h *Handlers) HandleGetStageSnapshot(writer http.ResponseWriter, request *http.Request) {
	key := request.URL.Query().Get("key")
	item, err := h.operation.GetStageSnapshot(request.Context(), key)
	if errors.Is(err, operationpkg.ErrStageSnapshotNotFound) {
		h.api.WriteSuccess(writer, &operationpkg.StageSnapshot{
			Key:       strings.TrimSpace(key),
			Snapshot:  nil,
			UpdatedAt: "",
		})
		return
	}
	if errors.Is(err, operationpkg.ErrInvalidStageSnapshot) {
		h.api.WriteFailure(writer, http.StatusBadRequest, "舞台快照参数错误")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleSaveStageSnapshot 保存会话舞台快照。
func (h *Handlers) HandleSaveStageSnapshot(writer http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(writer, request.Body, operationpkg.MaxStageSnapshotPayloadBytes+4096)
	var payload saveStageSnapshotRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.operation.SaveStageSnapshot(request.Context(), payload.Key, payload.Snapshot)
	if errors.Is(err, operationpkg.ErrInvalidStageSnapshot) {
		h.api.WriteFailure(writer, http.StatusBadRequest, "舞台快照参数错误")
		return
	}
	if errors.Is(err, operationpkg.ErrStageSnapshotTooLarge) {
		h.api.WriteFailure(writer, http.StatusRequestEntityTooLarge, "舞台快照过大")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleTouchStagePresence 刷新一个当前可见的舞台实例。
func (h *Handlers) HandleTouchStagePresence(writer http.ResponseWriter, request *http.Request) {
	var payload stagePresenceRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.operation.TouchStagePresence(request.Context(), payload.SessionKey, payload.ClientID)
	if errors.Is(err, operationpkg.ErrInvalidStagePresence) {
		h.api.WriteFailure(writer, http.StatusBadRequest, "舞台在线状态参数错误")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleCloseStagePresence 关闭当前浏览器对应的舞台实例。
func (h *Handlers) HandleCloseStagePresence(writer http.ResponseWriter, request *http.Request) {
	var payload stagePresenceRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.operation.CloseStagePresence(request.Context(), payload.SessionKey, payload.ClientID)
	if errors.Is(err, operationpkg.ErrInvalidStagePresence) {
		h.api.WriteFailure(writer, http.StatusBadRequest, "舞台在线状态参数错误")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleBrowserPage 返回经过公网地址校验和可嵌入化处理的 Navi 页面快照。
func (h *Handlers) HandleBrowserPage(writer http.ResponseWriter, request *http.Request) {
	document, err := h.operation.FetchBrowserPage(request.Context(), request.URL.Query().Get("url"))
	if err != nil {
		status, title, detail := browserPageError(err)
		writeBrowserPageError(writer, status, title, detail)
		return
	}

	setBrowserPageHeaders(writer.Header())
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Content-Location", document.URL)
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(document.HTML)
}

func browserPageError(err error) (int, string, string) {
	switch {
	case errors.Is(err, operationpkg.ErrInvalidBrowserPageURL):
		return http.StatusBadRequest, "无法打开这个地址", "Navi 只支持完整的 HTTP 或 HTTPS 网页地址。"
	case errors.Is(err, operationpkg.ErrBrowserPageAddressDenied):
		return http.StatusForbidden, "这个地址不可访问", "为保护本机和工作区，Navi 不会代理本地网络或受限地址。"
	case errors.Is(err, operationpkg.ErrBrowserPageTooLarge):
		return http.StatusRequestEntityTooLarge, "页面内容过大", "这个页面超出了 Navi 当前可载入的页面大小。"
	case errors.Is(err, operationpkg.ErrBrowserPageUnsupported):
		return http.StatusUnsupportedMediaType, "无法作为网页显示", "目标地址返回的不是 HTML 页面。"
	case errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout, "页面载入超时", "目标网站没有在限定时间内返回页面。"
	default:
		return http.StatusBadGateway, "页面暂时无法载入", "目标网站拒绝访问、连接失败或返回了无法处理的内容。"
	}
}

func writeBrowserPageError(writer http.ResponseWriter, status int, title string, detail string) {
	setBrowserPageHeaders(writer.Header())
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = browserPageErrorTemplate.Execute(writer, struct {
		Title  string
		Detail string
		Status int
	}{Title: title, Detail: detail, Status: status})
}

func setBrowserPageHeaders(header http.Header) {
	header.Set("Content-Type", "text/html; charset=utf-8")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set(
		"Content-Security-Policy",
		"default-src 'none'; base-uri http: https:; img-src http: https: data: blob:; media-src http: https: data: blob:; font-src http: https: data:; style-src http: https: 'unsafe-inline'; script-src http: https: 'unsafe-inline' 'unsafe-eval'; connect-src http: https: ws: wss:; frame-src http: https:; form-action http: https:; frame-ancestors 'self'; sandbox allow-downloads allow-forms allow-modals allow-pointer-lock allow-scripts",
	)
}
