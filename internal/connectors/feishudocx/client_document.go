package feishudocx

import (
	"context"
	"errors"
	"strings"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkdocx "github.com/larksuite/oapi-sdk-go/v3/service/docx/v1"
)

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
