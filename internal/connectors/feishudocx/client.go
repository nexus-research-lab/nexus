package feishudocx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkbitable "github.com/larksuite/oapi-sdk-go/v3/service/bitable/v1"
	larkdocx "github.com/larksuite/oapi-sdk-go/v3/service/docx/v1"
	larkdrive "github.com/larksuite/oapi-sdk-go/v3/service/drive/v1"
	larksheets "github.com/larksuite/oapi-sdk-go/v3/service/sheets/v3"
	larkwiki "github.com/larksuite/oapi-sdk-go/v3/service/wiki/v2"
)

const (
	defaultAPIBaseURL = "https://open.feishu.cn"
	defaultDocBaseURL = "https://feishu.cn"
	maxResponseBytes  = 4 * 1024 * 1024
	sdkAppID          = "nexus-feishu-docx"
)

// Client 封装飞书云文档 API。
type Client struct {
	apiBaseURL  string
	docBaseURL  string
	accessToken string
	httpClient  *http.Client
	docx        *larkdocx.V1
	drive       *larkdrive.V1
	sheets      *larksheets.V3
	bitable     *larkbitable.V1
	wiki        *larkwiki.V2
}

// NewClient 创建飞书云文档 API 客户端。
func NewClient(baseURL string, accessToken string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	baseURL = strings.TrimRight(firstNonEmpty(baseURL, defaultAPIBaseURL), "/")
	config := newSDKConfig(baseURL, httpClient)
	return &Client{
		apiBaseURL:  baseURL,
		docBaseURL:  defaultDocBaseURL,
		accessToken: strings.TrimSpace(accessToken),
		httpClient:  httpClient,
		docx:        larkdocx.New(config),
		drive:       larkdrive.New(config),
		sheets:      larksheets.New(config),
		bitable:     larkbitable.New(config),
		wiki:        larkwiki.New(config),
	}
}

// ResolveDocument 将 docx/wiki URL 或文档 ID 解析为实际 Docx document_id。
func (c *Client) ResolveDocument(ctx context.Context, raw string) (DocumentTarget, error) {
	target, err := ParseDocumentTarget(raw)
	if err != nil {
		return target, err
	}
	if target.DocumentID != "" {
		return target, nil
	}
	node, err := c.GetWikiNode(ctx, target.WikiToken)
	if err != nil {
		return target, err
	}
	if node.ObjType != "docx" {
		return target, fmt.Errorf("Wiki 节点类型 %q 暂不支持作为文档操作目标", node.ObjType)
	}
	target.DocumentID = node.ObjToken
	return target, nil
}

func (c *Client) ensureAccessToken() error {
	if c.accessToken == "" {
		return errors.New("飞书连接缺少 access token")
	}
	return nil
}

func (c *Client) authOption() larkcore.RequestOptionFunc {
	return larkcore.WithUserAccessToken(c.accessToken)
}

func (c *Client) doJSON(ctx context.Context, method string, apiPath string, payload any, out any) error {
	if err := c.ensureAccessToken(); err != nil {
		return err
	}
	req, err := c.newJSONRequest(ctx, method, apiPath, payload)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := readJSONResponse(resp)
	if err != nil {
		return err
	}
	return decodeJSONEnvelope(data, resp.Header.Get("X-Request-Id"), out)
}

func (c *Client) newJSONRequest(ctx context.Context, method string, apiPath string, payload any) (*http.Request, error) {
	body, err := marshalJSONBody(payload)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(c.apiBaseURL, "/") + apiPath
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	return req, nil
}

func marshalJSONBody(payload any) (io.Reader, error) {
	if payload == nil {
		return nil, nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func readJSONResponse(resp *http.Response) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxResponseBytes {
		return nil, errors.New("飞书 API 响应过大")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("飞书 API HTTP 错误 %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, nil
}

func decodeJSONEnvelope(data []byte, requestID string, out any) error {
	var envelope struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	if envelope.Code != 0 {
		return sdkCodeError(envelope.Code, envelope.Msg, requestID)
	}
	if out == nil || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil
	}
	return json.Unmarshal(envelope.Data, out)
}

func newSDKConfig(baseURL string, httpClient *http.Client) *larkcore.Config {
	config := &larkcore.Config{
		BaseUrl:          strings.TrimRight(baseURL, "/"),
		AppId:            sdkAppID,
		HttpClient:       limitedHTTPClient{base: httpClient, maxBytes: maxResponseBytes},
		EnableTokenCache: false,
	}
	larkcore.NewLogger(config)
	larkcore.NewSerialization(config)
	larkcore.NewHttpClient(config)
	return config
}

func sdkCodeError(code int, msg string, requestID string) error {
	if strings.TrimSpace(msg) == "" {
		msg = "unknown"
	}
	if strings.TrimSpace(requestID) != "" {
		return fmt.Errorf("飞书 API 返回错误 %d: %s (request_id: %s)", code, msg, requestID)
	}
	return fmt.Errorf("飞书 API 返回错误 %d: %s", code, msg)
}

type limitedHTTPClient struct {
	base     larkcore.HttpClient
	maxBytes int64
}

func (client limitedHTTPClient) Do(request *http.Request) (*http.Response, error) {
	resp, err := client.base.Do(request)
	if err != nil || resp == nil || resp.Body == nil || client.maxBytes <= 0 {
		return resp, err
	}
	resp.Body = &maxBytesReadCloser{body: resp.Body, maxBytes: client.maxBytes}
	return resp, nil
}

type maxBytesReadCloser struct {
	body     io.ReadCloser
	maxBytes int64
	read     int64
}

func (reader *maxBytesReadCloser) Read(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return reader.body.Read(buffer)
	}
	if reader.read >= reader.maxBytes {
		var probe [1]byte
		n, err := reader.body.Read(probe[:])
		if n > 0 {
			return 0, errors.New("飞书 API 响应过大")
		}
		return 0, err
	}
	remaining := reader.maxBytes - reader.read
	if int64(len(buffer)) > remaining {
		buffer = buffer[:remaining]
	}
	n, err := reader.body.Read(buffer)
	reader.read += int64(n)
	return n, err
}

func (reader *maxBytesReadCloser) Close() error {
	return reader.body.Close()
}
