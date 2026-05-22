package feishudocx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxResponseBytes = 4 * 1024 * 1024

// Client 封装飞书云文档 API。
type Client struct {
	baseURL     string
	docBaseURL  string
	accessToken string
	httpClient  *http.Client
}

// NewClient 创建飞书云文档 API 客户端。
func NewClient(baseURL string, accessToken string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	return &Client{
		baseURL:     strings.TrimRight(firstNonEmpty(baseURL, defaultAPIBaseURL), "/"),
		docBaseURL:  defaultDocBaseURL,
		accessToken: strings.TrimSpace(accessToken),
		httpClient:  httpClient,
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

// GetDocument 获取 Docx 文档元数据。
func (c *Client) GetDocument(ctx context.Context, documentID string) (*Document, error) {
	var data struct {
		Document Document `json:"document"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/open-apis/docx/v1/documents/"+url.PathEscape(documentID), nil, nil, &data); err != nil {
		return nil, err
	}
	return &data.Document, nil
}

// ListDocumentBlocks 拉取文档全部 Block。
func (c *Client) ListDocumentBlocks(ctx context.Context, documentID string) ([]Block, error) {
	var result []Block
	pageToken := ""
	for {
		query := url.Values{}
		query.Set("page_size", "500")
		query.Set("document_revision_id", "-1")
		if pageToken != "" {
			query.Set("page_token", pageToken)
		}
		var data struct {
			Items     []Block `json:"items"`
			PageToken string  `json:"page_token"`
			HasMore   bool    `json:"has_more"`
		}
		path := "/open-apis/docx/v1/documents/" + url.PathEscape(documentID) + "/blocks"
		if err := c.doJSON(ctx, http.MethodGet, path, query, nil, &data); err != nil {
			return nil, err
		}
		result = append(result, data.Items...)
		if !data.HasMore {
			break
		}
		pageToken = data.PageToken
		if pageToken == "" {
			break
		}
	}
	return result, nil
}

// ExportMarkdown 将飞书文档导出为 Markdown。
func (c *Client) ExportMarkdown(ctx context.Context, raw string, withBlockIDs bool) (*ExportMarkdownResult, error) {
	target, err := c.ResolveDocument(ctx, raw)
	if err != nil {
		return nil, err
	}
	document, err := c.GetDocument(ctx, target.DocumentID)
	if err != nil {
		return nil, err
	}
	blocks, err := c.ListDocumentBlocks(ctx, target.DocumentID)
	if err != nil {
		return nil, err
	}
	renderer := newMarkdownRenderer(blocks, document.Title, withBlockIDs)
	markdown := renderer.Render(target.DocumentID)
	return &ExportMarkdownResult{
		DocumentID:        target.DocumentID,
		Title:             document.Title,
		SourceType:        target.SourceType,
		Markdown:          markdown,
		BlockCount:        len(blocks),
		UnsupportedBlocks: renderer.UnsupportedBlocks(),
	}, nil
}

// CreateDocument 创建文档，并可直接写入 Markdown。
func (c *Client) CreateDocument(ctx context.Context, title string, markdown string, folderToken string) (*CreateDocumentResult, error) {
	body := map[string]any{"title": strings.TrimSpace(title)}
	if strings.TrimSpace(folderToken) != "" {
		body["folder_token"] = strings.TrimSpace(folderToken)
	}
	var data struct {
		Document Document `json:"document"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/open-apis/docx/v1/documents", nil, body, &data); err != nil {
		return nil, err
	}
	result := &CreateDocumentResult{
		DocumentID: data.Document.DocumentID,
		URL:        c.docBaseURL + "/docx/" + data.Document.DocumentID,
		Title:      firstNonEmpty(data.Document.Title, title),
	}
	if strings.TrimSpace(markdown) == "" {
		return result, nil
	}
	appendResult, err := c.AppendMarkdown(ctx, data.Document.DocumentID, markdown)
	if err != nil {
		return nil, err
	}
	result.CreatedBlocks = appendResult.CreatedBlocks
	return result, nil
}

// ConvertMarkdown 使用飞书原生转换接口把 Markdown 转成文档 Block。
func (c *Client) ConvertMarkdown(ctx context.Context, markdown string) (*ConvertResult, error) {
	body := map[string]any{
		"content_type": "markdown",
		"content":      markdown,
	}
	var data ConvertResult
	if err := c.doJSON(ctx, http.MethodPost, "/open-apis/docx/v1/documents/blocks/convert", nil, body, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// AppendMarkdown 向文档根节点追加 Markdown 内容。
func (c *Client) AppendMarkdown(ctx context.Context, documentID string, markdown string) (*AppendResult, error) {
	converted, err := c.ConvertMarkdown(ctx, markdown)
	if err != nil {
		return nil, err
	}
	return c.AppendConvertedBlocks(ctx, documentID, *converted)
}

// AppendConvertedBlocks 使用 descendant 接口按转换后的 block_id 关系写入块。
func (c *Client) AppendConvertedBlocks(ctx context.Context, documentID string, converted ConvertResult) (*AppendResult, error) {
	result := &AppendResult{}
	if len(converted.FirstLevelBlockIDs) == 0 || len(converted.Blocks) == 0 {
		return result, nil
	}
	for _, ids := range chunkStrings(converted.FirstLevelBlockIDs, 50) {
		descendants := filterDescendants(converted.Blocks, ids)
		if len(descendants) == 0 {
			continue
		}
		body := map[string]any{
			"children_id": ids,
			"index":       -1,
			"descendants": descendants,
		}
		query := url.Values{}
		query.Set("document_revision_id", "-1")
		var data AppendResult
		path := "/open-apis/docx/v1/documents/" + url.PathEscape(documentID) + "/blocks/" + url.PathEscape(documentID) + "/descendant"
		if err := c.doJSON(ctx, http.MethodPost, path, query, body, &data); err != nil {
			return nil, err
		}
		result.Children = append(result.Children, data.Children...)
		result.BlockIDRelations = append(result.BlockIDRelations, data.BlockIDRelations...)
		result.DocumentRevisionID = data.DocumentRevisionID
		result.CreatedBlocks += len(descendants)
	}
	return result, nil
}

// UpdateTextBlock 更新普通文本类 Block 内容。
func (c *Client) UpdateTextBlock(ctx context.Context, documentID string, blockID string, content string) (Block, error) {
	body := map[string]any{
		"text": map[string]any{
			"elements": []map[string]any{
				{"text_run": map[string]any{"content": content}},
			},
		},
	}
	query := url.Values{}
	query.Set("document_revision_id", "-1")
	var data struct {
		Block Block `json:"block"`
	}
	path := "/open-apis/docx/v1/documents/" + url.PathEscape(documentID) + "/blocks/" + url.PathEscape(blockID)
	if err := c.doJSON(ctx, http.MethodPatch, path, query, body, &data); err != nil {
		return nil, err
	}
	return data.Block, nil
}

// ListDriveFiles 列出云空间文件。
func (c *Client) ListDriveFiles(ctx context.Context, folderToken string, pageToken string, pageSize int, orderBy string, direction string, option string) (*DriveListResult, error) {
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}
	query := url.Values{}
	query.Set("page_size", fmt.Sprintf("%d", pageSize))
	if strings.TrimSpace(folderToken) != "" {
		query.Set("folder_token", strings.TrimSpace(folderToken))
	}
	if strings.TrimSpace(pageToken) != "" {
		query.Set("page_token", strings.TrimSpace(pageToken))
	}
	if strings.TrimSpace(orderBy) != "" {
		query.Set("order_by", strings.TrimSpace(orderBy))
	}
	if strings.TrimSpace(direction) != "" {
		query.Set("direction", strings.TrimSpace(direction))
	}
	if strings.TrimSpace(option) != "" {
		query.Set("option", strings.TrimSpace(option))
	}
	var data struct {
		Files         []map[string]any `json:"files"`
		NextPageToken string           `json:"next_page_token"`
		HasMore       bool             `json:"has_more"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/open-apis/drive/v1/files", query, nil, &data); err != nil {
		return nil, err
	}
	return &DriveListResult{
		Files:         data.Files,
		NextPageToken: data.NextPageToken,
		HasMore:       data.HasMore,
	}, nil
}

func (c *Client) doJSON(ctx context.Context, method string, path string, query url.Values, body any, out any) error {
	if c.accessToken == "" {
		return errors.New("飞书连接缺少 access token")
	}
	fullURL, err := c.buildURL(path, query)
	if err != nil {
		return err
	}
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return err
	}
	if len(payload) > maxResponseBytes {
		return errors.New("飞书 API 响应过大")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("飞书 API HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	var envelope struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err = json.Unmarshal(payload, &envelope); err != nil {
		return err
	}
	if envelope.Code != 0 {
		return fmt.Errorf("飞书 API 返回错误 %d: %s", envelope.Code, envelope.Msg)
	}
	if out == nil || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil
	}
	return json.Unmarshal(envelope.Data, out)
}

func (c *Client) buildURL(path string, query url.Values) (string, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return "", errors.New("飞书 API base URL 格式不正确")
	}
	relative := &url.URL{Path: strings.TrimRight(base.Path, "/") + "/" + strings.TrimLeft(path, "/")}
	fullURL := base.ResolveReference(relative)
	fullURL.RawQuery = query.Encode()
	return fullURL.String(), nil
}

func filterDescendants(blocks []Block, firstLevelIDs []string) []Block {
	byID := map[string]Block{}
	for _, block := range blocks {
		if id := blockID(block); id != "" {
			byID[id] = block
		}
	}
	seen := map[string]bool{}
	var result []Block
	var walk func(string)
	walk = func(id string) {
		if seen[id] {
			return
		}
		block, ok := byID[id]
		if !ok {
			return
		}
		seen[id] = true
		result = append(result, block)
		for _, childID := range blockChildren(block) {
			walk(childID)
		}
	}
	for _, id := range firstLevelIDs {
		walk(id)
	}
	return result
}

func chunkStrings(values []string, size int) [][]string {
	if size <= 0 || len(values) == 0 {
		return nil
	}
	var result [][]string
	for start := 0; start < len(values); start += size {
		end := start + size
		if end > len(values) {
			end = len(values)
		}
		result = append(result, values[start:end])
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
