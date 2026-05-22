package feishudocx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkdocx "github.com/larksuite/oapi-sdk-go/v3/service/docx/v1"
	larkdrive "github.com/larksuite/oapi-sdk-go/v3/service/drive/v1"
	larkwiki "github.com/larksuite/oapi-sdk-go/v3/service/wiki/v2"
)

const (
	maxResponseBytes = 4 * 1024 * 1024
	sdkAppID         = "nexus-feishu-docx"
)

// Client 封装飞书云文档 API。
type Client struct {
	docBaseURL  string
	accessToken string
	docx        *larkdocx.V1
	drive       *larkdrive.V1
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
		docBaseURL:  defaultDocBaseURL,
		accessToken: strings.TrimSpace(accessToken),
		docx:        larkdocx.New(config),
		drive:       larkdrive.New(config),
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

// GetDocument 获取 Docx 文档元数据。
func (c *Client) GetDocument(ctx context.Context, documentID string) (*Document, error) {
	documentID = strings.TrimSpace(documentID)
	if documentID == "" {
		return nil, errors.New("document_id 不能为空")
	}
	if err := c.ensureAccessToken(); err != nil {
		return nil, err
	}
	req := larkdocx.NewGetDocumentReqBuilder().
		DocumentId(documentID).
		Build()
	resp, err := c.docx.Document.Get(ctx, req, c.authOption())
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, sdkCodeError(resp.Code, resp.Msg, resp.RequestId())
	}
	if resp.Data == nil || resp.Data.Document == nil {
		return nil, errors.New("飞书文档响应缺少 document")
	}
	document := resp.Data.Document
	return &Document{
		DocumentID: larkcore.StringValue(document.DocumentId),
		RevisionID: int64(larkcore.IntValue(document.RevisionId)),
		Title:      larkcore.StringValue(document.Title),
	}, nil
}

// ListDocumentBlocks 拉取文档全部 Block。
func (c *Client) ListDocumentBlocks(ctx context.Context, documentID string) ([]Block, error) {
	documentID = strings.TrimSpace(documentID)
	if documentID == "" {
		return nil, errors.New("document_id 不能为空")
	}
	if err := c.ensureAccessToken(); err != nil {
		return nil, err
	}
	var result []Block
	pageToken := ""
	for {
		builder := larkdocx.NewListDocumentBlockReqBuilder().
			DocumentId(documentID).
			PageSize(500).
			DocumentRevisionId(-1)
		if pageToken != "" {
			builder.PageToken(pageToken)
		}
		resp, err := c.docx.DocumentBlock.List(ctx, builder.Build(), c.authOption())
		if err != nil {
			return nil, err
		}
		if !resp.Success() {
			return nil, sdkCodeError(resp.Code, resp.Msg, resp.RequestId())
		}
		if resp.Data == nil {
			break
		}
		blocks, err := sdkBlocksToMaps(resp.Data.Items)
		if err != nil {
			return nil, err
		}
		result = append(result, blocks...)
		if !larkcore.BoolValue(resp.Data.HasMore) {
			break
		}
		pageToken = larkcore.StringValue(resp.Data.PageToken)
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
	if err := c.ensureAccessToken(); err != nil {
		return nil, err
	}
	bodyBuilder := larkdocx.NewCreateDocumentReqBodyBuilder().
		Title(strings.TrimSpace(title))
	if strings.TrimSpace(folderToken) != "" {
		bodyBuilder.FolderToken(strings.TrimSpace(folderToken))
	}
	req := larkdocx.NewCreateDocumentReqBuilder().
		Body(bodyBuilder.Build()).
		Build()
	resp, err := c.docx.Document.Create(ctx, req, c.authOption())
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, sdkCodeError(resp.Code, resp.Msg, resp.RequestId())
	}
	if resp.Data == nil || resp.Data.Document == nil {
		return nil, errors.New("飞书文档响应缺少 document")
	}
	document := resp.Data.Document
	documentID := larkcore.StringValue(document.DocumentId)
	result := &CreateDocumentResult{
		DocumentID: documentID,
		URL:        c.docBaseURL + "/docx/" + documentID,
		Title:      firstNonEmpty(larkcore.StringValue(document.Title), title),
	}
	if strings.TrimSpace(markdown) == "" {
		return result, nil
	}
	appendResult, err := c.AppendMarkdown(ctx, documentID, markdown)
	if err != nil {
		return nil, err
	}
	result.CreatedBlocks = appendResult.CreatedBlocks
	return result, nil
}

// ConvertMarkdown 使用飞书原生转换接口把 Markdown 转成文档 Block。
func (c *Client) ConvertMarkdown(ctx context.Context, markdown string) (*ConvertResult, error) {
	if err := c.ensureAccessToken(); err != nil {
		return nil, err
	}
	body := larkdocx.NewConvertDocumentReqBodyBuilder().
		ContentType("markdown").
		Content(markdown).
		Build()
	req := larkdocx.NewConvertDocumentReqBuilder().
		Body(body).
		Build()
	resp, err := c.docx.Document.Convert(ctx, req, c.authOption())
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, sdkCodeError(resp.Code, resp.Msg, resp.RequestId())
	}
	if resp.Data == nil {
		return &ConvertResult{}, nil
	}
	blocks, err := sdkBlocksToMaps(resp.Data.Blocks)
	if err != nil {
		return nil, err
	}
	return &ConvertResult{
		FirstLevelBlockIDs: resp.Data.FirstLevelBlockIds,
		Blocks:             blocks,
	}, nil
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
	documentID = strings.TrimSpace(documentID)
	if documentID == "" {
		return nil, errors.New("document_id 不能为空")
	}
	if err := c.ensureAccessToken(); err != nil {
		return nil, err
	}
	result := &AppendResult{}
	if len(converted.FirstLevelBlockIDs) == 0 || len(converted.Blocks) == 0 {
		return result, nil
	}
	for _, ids := range chunkStrings(converted.FirstLevelBlockIDs, 50) {
		descendants := filterDescendants(converted.Blocks, ids)
		if len(descendants) == 0 {
			continue
		}
		sdkDescendants, err := mapsToSDKBlocks(descendants)
		if err != nil {
			return nil, err
		}
		body := larkdocx.NewCreateDocumentBlockDescendantReqBodyBuilder().
			ChildrenId(ids).
			Index(-1).
			Descendants(sdkDescendants).
			Build()
		req := larkdocx.NewCreateDocumentBlockDescendantReqBuilder().
			DocumentId(documentID).
			BlockId(documentID).
			DocumentRevisionId(-1).
			Body(body).
			Build()
		resp, err := c.docx.DocumentBlockDescendant.Create(ctx, req, c.authOption())
		if err != nil {
			return nil, err
		}
		if !resp.Success() {
			return nil, sdkCodeError(resp.Code, resp.Msg, resp.RequestId())
		}
		if resp.Data == nil {
			continue
		}
		children, err := sdkBlocksToMaps(resp.Data.Children)
		if err != nil {
			return nil, err
		}
		blockIDRelations, err := sdkObjectsToMaps(resp.Data.BlockIdRelations)
		if err != nil {
			return nil, err
		}
		result.Children = append(result.Children, children...)
		result.BlockIDRelations = append(result.BlockIDRelations, blockIDRelations...)
		result.DocumentRevisionID = int64(larkcore.IntValue(resp.Data.DocumentRevisionId))
		result.CreatedBlocks += len(descendants)
	}
	return result, nil
}

// UpdateTextBlock 更新普通文本类 Block 内容。
func (c *Client) UpdateTextBlock(ctx context.Context, documentID string, blockID string, content string) (Block, error) {
	documentID = strings.TrimSpace(documentID)
	blockID = strings.TrimSpace(blockID)
	if documentID == "" || blockID == "" {
		return nil, errors.New("document_id 和 block_id 不能为空")
	}
	if err := c.ensureAccessToken(); err != nil {
		return nil, err
	}
	textRun := larkdocx.NewTextRunBuilder().
		Content(content).
		Build()
	textElement := larkdocx.NewTextElementBuilder().
		TextRun(textRun).
		Build()
	updateText := larkdocx.NewUpdateTextRequestBuilder().
		Elements([]*larkdocx.TextElement{textElement}).
		Build()
	updateBlock := larkdocx.NewUpdateBlockRequestBuilder().
		UpdateText(updateText).
		Build()
	req := larkdocx.NewPatchDocumentBlockReqBuilder().
		DocumentId(documentID).
		BlockId(blockID).
		DocumentRevisionId(-1).
		UpdateBlockRequest(updateBlock).
		Build()
	resp, err := c.docx.DocumentBlock.Patch(ctx, req, c.authOption())
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, sdkCodeError(resp.Code, resp.Msg, resp.RequestId())
	}
	if resp.Data == nil || resp.Data.Block == nil {
		return nil, errors.New("飞书文档响应缺少 block")
	}
	block, err := sdkBlockToMap(resp.Data.Block)
	if err != nil {
		return nil, err
	}
	return block, nil
}

// ListDriveFiles 列出云空间文件。
func (c *Client) ListDriveFiles(ctx context.Context, folderToken string, pageToken string, pageSize int, orderBy string, direction string, option string) (*DriveListResult, error) {
	if err := c.ensureAccessToken(); err != nil {
		return nil, err
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}
	builder := larkdrive.NewListFileReqBuilder().
		PageSize(pageSize)
	if strings.TrimSpace(folderToken) != "" {
		builder.FolderToken(strings.TrimSpace(folderToken))
	}
	if strings.TrimSpace(pageToken) != "" {
		builder.PageToken(strings.TrimSpace(pageToken))
	}
	if strings.TrimSpace(orderBy) != "" {
		builder.OrderBy(strings.TrimSpace(orderBy))
	}
	if strings.TrimSpace(direction) != "" {
		builder.Direction(strings.TrimSpace(direction))
	}
	if strings.TrimSpace(option) != "" {
		builder.Option(strings.TrimSpace(option))
	}
	resp, err := c.drive.File.List(ctx, builder.Build(), c.authOption())
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, sdkCodeError(resp.Code, resp.Msg, resp.RequestId())
	}
	if resp.Data == nil {
		return &DriveListResult{Files: []map[string]any{}}, nil
	}
	files, err := sdkObjectsToMaps(resp.Data.Files)
	if err != nil {
		return nil, err
	}
	return &DriveListResult{
		Files:         files,
		NextPageToken: larkcore.StringValue(resp.Data.NextPageToken),
		HasMore:       larkcore.BoolValue(resp.Data.HasMore),
	}, nil
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

func sdkBlocksToMaps(blocks []*larkdocx.Block) ([]Block, error) {
	if len(blocks) == 0 {
		return []Block{}, nil
	}
	result := make([]Block, 0, len(blocks))
	for _, block := range blocks {
		item, err := sdkBlockToMap(block)
		if err != nil {
			return nil, err
		}
		if item != nil {
			result = append(result, item)
		}
	}
	return result, nil
}

func sdkBlockToMap(block *larkdocx.Block) (Block, error) {
	if block == nil {
		return nil, nil
	}
	payload, err := json.Marshal(block)
	if err != nil {
		return nil, err
	}
	var result Block
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func mapsToSDKBlocks(blocks []Block) ([]*larkdocx.Block, error) {
	if len(blocks) == 0 {
		return []*larkdocx.Block{}, nil
	}
	payload, err := json.Marshal(blocks)
	if err != nil {
		return nil, err
	}
	var result []*larkdocx.Block
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func sdkObjectsToMaps(value any) ([]map[string]any, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if string(payload) == "null" {
		return []map[string]any{}, nil
	}
	var result []map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, err
	}
	if result == nil {
		return []map[string]any{}, nil
	}
	return result, nil
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
