package feishudocx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkbitable "github.com/larksuite/oapi-sdk-go/v3/service/bitable/v1"
)

// BitableTarget 是从 URL 或 token 里解析出的多维表格目标。
type BitableTarget struct {
	AppToken string `json:"app_token"`
	TableID  string `json:"table_id,omitempty"`
	Raw      string `json:"raw"`
}

// BitableTableListResult 表示多维表格下的数据表列表。
type BitableTableListResult struct {
	AppToken      string           `json:"app_token"`
	Tables        []map[string]any `json:"tables"`
	PageToken     string           `json:"page_token,omitempty"`
	NextPageToken string           `json:"next_page_token,omitempty"`
	HasMore       bool             `json:"has_more"`
	Total         int              `json:"total,omitempty"`
}

// BitableFieldListResult 表示多维表格字段列表。
type BitableFieldListResult struct {
	AppToken      string           `json:"app_token"`
	TableID       string           `json:"table_id"`
	Fields        []map[string]any `json:"fields"`
	PageToken     string           `json:"page_token,omitempty"`
	NextPageToken string           `json:"next_page_token,omitempty"`
	HasMore       bool             `json:"has_more"`
	Total         int              `json:"total,omitempty"`
}

// BitableRecordListResult 表示多维表格记录列表。
type BitableRecordListResult struct {
	AppToken      string           `json:"app_token"`
	TableID       string           `json:"table_id"`
	Records       []map[string]any `json:"records"`
	PageToken     string           `json:"page_token,omitempty"`
	NextPageToken string           `json:"next_page_token,omitempty"`
	HasMore       bool             `json:"has_more"`
	Total         int              `json:"total,omitempty"`
}

// ParseBitableTarget 从飞书多维表格 URL 或 token 中提取 app_token 和 table_id。
func ParseBitableTarget(raw string) (BitableTarget, error) {
	value := strings.TrimSpace(raw)
	target := BitableTarget{Raw: value}
	if value == "" {
		return target, errors.New("飞书 Bitable URL 或 app_token 不能为空")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		if strings.Contains(value, "/") {
			return target, fmt.Errorf("飞书 Bitable URL 格式不正确: %s", value)
		}
		target.AppToken = value
		return target, nil
	}
	segments := splitPath(parsed.Path)
	for index, segment := range segments {
		if segment == "base" && index+1 < len(segments) {
			target.AppToken = strings.TrimSpace(segments[index+1])
			query := parsed.Query()
			target.TableID = firstNonEmpty(query.Get("table"), query.Get("table_id"))
			return target, nil
		}
	}
	return target, fmt.Errorf("URL 中未找到飞书 Bitable app_token: %s", value)
}

// ListBitableTables 列出多维表格应用下的数据表。
func (c *Client) ListBitableTables(ctx context.Context, raw string, pageToken string, pageSize int) (*BitableTableListResult, error) {
	target, err := ParseBitableTarget(raw)
	if err != nil {
		return nil, err
	}
	if err := c.ensureAccessToken(); err != nil {
		return nil, err
	}
	builder := larkbitable.NewListAppTableReqBuilder().
		AppToken(target.AppToken).
		PageSize(normalizePageSize(pageSize, 100, 100))
	if strings.TrimSpace(pageToken) != "" {
		builder.PageToken(strings.TrimSpace(pageToken))
	}
	resp, err := c.bitable.AppTable.List(ctx, builder.Build(), c.authOption())
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, sdkCodeError(resp.Code, resp.Msg, resp.RequestId())
	}
	result := &BitableTableListResult{AppToken: target.AppToken, Tables: []map[string]any{}}
	if resp.Data == nil {
		return result, nil
	}
	tables, err := sdkObjectsToMaps(resp.Data.Items)
	if err != nil {
		return nil, err
	}
	result.Tables = tables
	result.NextPageToken = larkcore.StringValue(resp.Data.PageToken)
	result.PageToken = result.NextPageToken
	result.HasMore = larkcore.BoolValue(resp.Data.HasMore)
	result.Total = larkcore.IntValue(resp.Data.Total)
	return result, nil
}

// ListBitableFields 列出多维表格字段。
func (c *Client) ListBitableFields(ctx context.Context, raw string, tableID string, viewID string, pageToken string, pageSize int) (*BitableFieldListResult, error) {
	target, err := ParseBitableTarget(raw)
	if err != nil {
		return nil, err
	}
	tableID = firstNonEmpty(tableID, target.TableID)
	if tableID == "" {
		return nil, errors.New("table_id 不能为空；可传 base URL 或显式传 table_id")
	}
	if err := c.ensureAccessToken(); err != nil {
		return nil, err
	}
	builder := larkbitable.NewListAppTableFieldReqBuilder().
		AppToken(target.AppToken).
		TableId(tableID).
		TextFieldAsArray(true).
		PageSize(normalizePageSize(pageSize, 100, 100))
	if strings.TrimSpace(viewID) != "" {
		builder.ViewId(strings.TrimSpace(viewID))
	}
	if strings.TrimSpace(pageToken) != "" {
		builder.PageToken(strings.TrimSpace(pageToken))
	}
	resp, err := c.bitable.AppTableField.List(ctx, builder.Build(), c.authOption())
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, sdkCodeError(resp.Code, resp.Msg, resp.RequestId())
	}
	result := &BitableFieldListResult{AppToken: target.AppToken, TableID: tableID, Fields: []map[string]any{}}
	if resp.Data == nil {
		return result, nil
	}
	fields, err := sdkObjectsToMaps(resp.Data.Items)
	if err != nil {
		return nil, err
	}
	result.Fields = fields
	result.NextPageToken = larkcore.StringValue(resp.Data.PageToken)
	result.PageToken = result.NextPageToken
	result.HasMore = larkcore.BoolValue(resp.Data.HasMore)
	result.Total = larkcore.IntValue(resp.Data.Total)
	return result, nil
}

// ListBitableRecords 列出多维表格记录，并支持字段、视图、筛选和排序。
func (c *Client) ListBitableRecords(ctx context.Context, raw string, tableID string, viewID string, fieldNames []string, filter string, sort string, pageToken string, pageSize int, automaticFields bool) (*BitableRecordListResult, error) {
	target, err := ParseBitableTarget(raw)
	if err != nil {
		return nil, err
	}
	tableID = firstNonEmpty(tableID, target.TableID)
	if tableID == "" {
		return nil, errors.New("table_id 不能为空；可传 base URL 或显式传 table_id")
	}
	if err := c.ensureAccessToken(); err != nil {
		return nil, err
	}
	builder := larkbitable.NewListAppTableRecordReqBuilder().
		AppToken(target.AppToken).
		TableId(tableID).
		TextFieldAsArray(true).
		PageSize(normalizePageSize(pageSize, 100, 500))
	if strings.TrimSpace(viewID) != "" {
		builder.ViewId(strings.TrimSpace(viewID))
	}
	if len(fieldNames) > 0 {
		payload, err := json.Marshal(fieldNames)
		if err != nil {
			return nil, err
		}
		builder.FieldNames(string(payload))
	}
	if strings.TrimSpace(filter) != "" {
		builder.Filter(strings.TrimSpace(filter))
	}
	if strings.TrimSpace(sort) != "" {
		builder.Sort(strings.TrimSpace(sort))
	}
	if automaticFields {
		builder.AutomaticFields(true)
	}
	if strings.TrimSpace(pageToken) != "" {
		builder.PageToken(strings.TrimSpace(pageToken))
	}
	resp, err := c.bitable.AppTableRecord.List(ctx, builder.Build(), c.authOption())
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, sdkCodeError(resp.Code, resp.Msg, resp.RequestId())
	}
	result := &BitableRecordListResult{AppToken: target.AppToken, TableID: tableID, Records: []map[string]any{}}
	if resp.Data == nil {
		return result, nil
	}
	records, err := sdkObjectsToMaps(resp.Data.Items)
	if err != nil {
		return nil, err
	}
	result.Records = records
	result.NextPageToken = larkcore.StringValue(resp.Data.PageToken)
	result.PageToken = result.NextPageToken
	result.HasMore = larkcore.BoolValue(resp.Data.HasMore)
	result.Total = larkcore.IntValue(resp.Data.Total)
	return result, nil
}
