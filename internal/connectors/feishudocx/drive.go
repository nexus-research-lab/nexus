package feishudocx

import (
	"context"
	"strings"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkdrive "github.com/larksuite/oapi-sdk-go/v3/service/drive/v1"
)

// DriveListResult 表示云空间文件列表。
type DriveListResult struct {
	Files         []map[string]any `json:"files"`
	NextPageToken string           `json:"next_page_token,omitempty"`
	HasMore       bool             `json:"has_more"`
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
