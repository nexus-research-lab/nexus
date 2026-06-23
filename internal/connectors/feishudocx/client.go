package feishudocx

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	larkbitable "github.com/larksuite/oapi-sdk-go/v3/service/bitable/v1"
	larkdocx "github.com/larksuite/oapi-sdk-go/v3/service/docx/v1"
	larkdrive "github.com/larksuite/oapi-sdk-go/v3/service/drive/v1"
	larksheets "github.com/larksuite/oapi-sdk-go/v3/service/sheets/v3"
	larkwiki "github.com/larksuite/oapi-sdk-go/v3/service/wiki/v2"
)

const (
	maxResponseBytes = 4 * 1024 * 1024
	sdkAppID         = "nexus-feishu-docx"
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
