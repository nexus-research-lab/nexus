package tool

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/mcp/connectors/contract"
)

const maxResponseBytes = 256 * 1024

var connectorCallHTTPClient = &http.Client{Timeout: 20 * time.Second}

func call(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "connector_call",
		Description: "使用已连接 connector 的 access token 调用 provider REST API。",
		SearchHint:  searchHintConnectorCall,
		InputSchema: connectorCallSchema(),
		Annotations: &sdktool.ToolAnnotations{OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			input, err := parseCallInput(args)
			if err != nil {
				return errorResult(err), nil
			}
			snapshot, err := svc.LoadActiveConnection(ctx, sctx.OwnerUserID, input.ConnectorID)
			if err != nil {
				return errorResult(err), nil
			}
			if snapshot == nil {
				return errorResult(errors.New("connector 未连接")), nil
			}
			output, err := executeCall(ctx, snapshot.APIBaseURL, snapshot.AccessToken, snapshot.ShopDomain, snapshot.Extra, input)
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(output), nil
		},
	}
}

type callInput struct {
	ConnectorID string
	Method      string
	Path        string
	Query       map[string]string
	Headers     map[string]string
	Body        map[string]any
}

func parseCallInput(args map[string]any) (callInput, error) {
	input := callInput{
		ConnectorID: strings.TrimSpace(stringValue(args["connector_id"])),
		Method:      strings.ToUpper(strings.TrimSpace(stringValue(args["method"]))),
		Path:        strings.TrimSpace(stringValue(args["path"])),
		Query:       stringMap(args["query"]),
		Headers:     stringMap(args["headers"]),
		Body:        objectMap(args["body"]),
	}
	if input.ConnectorID == "" || input.Method == "" || input.Path == "" {
		return input, errors.New("connector_id、method、path 不能为空")
	}
	if !strings.HasPrefix(input.Path, "/") {
		return input, errors.New("path 必须以 / 开头")
	}
	switch input.Method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return input, errors.New("method 不支持")
	}
	return input, nil
}

func executeCall(ctx context.Context, apiBaseURL, accessToken, shopDomain string, extra map[string]string, input callInput) (map[string]any, error) {
	fullURL, err := resolveConnectorCallURL(apiBaseURL, shopDomain, extra, input)
	if err != nil {
		return nil, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	request, err := newConnectorCallRequest(requestCtx, fullURL, accessToken, input)
	if err != nil {
		return nil, err
	}
	response, err := connectorCallHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	return readConnectorCallResponse(response)
}

func resolveConnectorCallURL(
	apiBaseURL string,
	shopDomain string,
	extra map[string]string,
	input callInput,
) (*url.URL, error) {
	base := strings.TrimSpace(apiBaseURL)
	shop := cmp.Or(
		strings.TrimSpace(shopDomain),
		strings.TrimSpace(extra["shop"]),
		strings.TrimSpace(extra["shop_domain"]),
	)
	if strings.Contains(base, "{shop}") {
		if shop == "" {
			return nil, errors.New("Shopify connector 缺少 shop")
		}
		base = strings.ReplaceAll(base, "{shop}", shop)
	}
	parsedBase, err := url.Parse(base)
	if err != nil || parsedBase.Scheme == "" || parsedBase.Host == "" {
		return nil, errors.New("connector API base URL 格式不正确")
	}
	if err = validateOutboundBaseURL(parsedBase); err != nil {
		return nil, err
	}
	fullURL := parsedBase.ResolveReference(&url.URL{Path: strings.TrimRight(parsedBase.Path, "/") + input.Path})
	query := fullURL.Query()
	for key, value := range input.Query {
		query.Set(key, value)
	}
	fullURL.RawQuery = query.Encode()
	return fullURL, nil
}

func newConnectorCallRequest(
	ctx context.Context,
	fullURL *url.URL,
	accessToken string,
	input callInput,
) (*http.Request, error) {
	var body io.Reader
	if input.Body != nil {
		payload, err := json.Marshal(input.Body)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(payload)
	}
	request, err := http.NewRequestWithContext(ctx, input.Method, fullURL.String(), body)
	if err != nil {
		return nil, err
	}
	if input.Body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, value := range input.Headers {
		if strings.EqualFold(key, "Authorization") {
			continue
		}
		request.Header.Set(key, value)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	return request, nil
}

func readConnectorCallResponse(response *http.Response) (map[string]any, error) {
	limited := io.LimitReader(response.Body, maxResponseBytes+1)
	payload, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	truncated := len(payload) > maxResponseBytes
	if truncated {
		payload = payload[:maxResponseBytes]
	}
	return map[string]any{
		"status":     response.StatusCode,
		"headers":    responseHeaders(response.Header),
		"body":       string(payload),
		"_truncated": truncated,
	}, nil
}

func validateOutboundBaseURL(parsed *url.URL) error {
	if parsed.Scheme == "https" {
		return nil
	}
	host := parsed.Hostname()
	if parsed.Scheme == "http" {
		parsedIP := net.ParseIP(host)
		if host == "localhost" || (parsedIP != nil && parsedIP.IsLoopback()) {
			return nil
		}
	}
	return fmt.Errorf("connector API base URL 只允许 https 或 localhost 调试地址")
}

func connectorCallSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"connector_id", "method", "path"},
		"properties": map[string]any{
			"connector_id": map[string]any{"type": "string"},
			"method":       map[string]any{"type": "string", "enum": []string{"GET", "POST", "PUT", "PATCH", "DELETE"}},
			"path":         map[string]any{"type": "string", "description": "相对 api_base_url 的路径，必须以 / 开头"},
			"query":        map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
			"headers":      map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
			"body":         map[string]any{"type": "object"},
		},
	}
}

func responseHeaders(headers http.Header) map[string]string {
	result := map[string]string{}
	for key, values := range headers {
		result[key] = strings.Join(values, ",")
	}
	return result
}

func stringValue(value any) string {
	typed, _ := value.(string)
	return typed
}

func stringMap(value any) map[string]string {
	raw, ok := value.(map[string]any)
	if !ok {
		return map[string]string{}
	}
	result := map[string]string{}
	for key, value := range raw {
		if s, ok := value.(string); ok {
			result[key] = s
		}
	}
	return result
}

func objectMap(value any) map[string]any {
	raw, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return raw
}
