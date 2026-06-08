package feishudocx

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	larksheets "github.com/larksuite/oapi-sdk-go/v3/service/sheets/v3"
)

// ParseSheetTarget 从飞书电子表格 URL 或 token 中提取 spreadsheet_token 和 sheet_id。
func ParseSheetTarget(raw string) (SheetTarget, error) {
	value := strings.TrimSpace(raw)
	target := SheetTarget{Raw: value}
	if value == "" {
		return target, errors.New("飞书 Sheet URL 或 spreadsheet_token 不能为空")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		if strings.Contains(value, "/") {
			return target, fmt.Errorf("飞书 Sheet URL 格式不正确: %s", value)
		}
		target.SpreadsheetToken = value
		return target, nil
	}
	segments := splitPath(parsed.Path)
	for index, segment := range segments {
		if segment == "sheets" && index+1 < len(segments) {
			target.SpreadsheetToken = strings.TrimSpace(segments[index+1])
			target.SheetID = firstNonEmpty(parsed.Query().Get("sheet"), parsed.Query().Get("sheet_id"))
			return target, nil
		}
	}
	return target, fmt.Errorf("URL 中未找到飞书 Sheet token: %s", value)
}

// ListSheets 列出电子表格中的工作表。
func (c *Client) ListSheets(ctx context.Context, raw string) (*SheetListResult, error) {
	target, err := ParseSheetTarget(raw)
	if err != nil {
		return nil, err
	}
	if err := c.ensureAccessToken(); err != nil {
		return nil, err
	}
	req := larksheets.NewQuerySpreadsheetSheetReqBuilder().
		SpreadsheetToken(target.SpreadsheetToken).
		Build()
	resp, err := c.sheets.SpreadsheetSheet.Query(ctx, req, c.authOption())
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, sdkCodeError(resp.Code, resp.Msg, resp.RequestId())
	}
	result := &SheetListResult{SpreadsheetToken: target.SpreadsheetToken, Sheets: []map[string]any{}}
	if resp.Data == nil {
		return result, nil
	}
	sheets, err := sdkObjectsToMaps(resp.Data.Sheets)
	if err != nil {
		return nil, err
	}
	result.Sheets = sheets
	return result, nil
}

// ReadSheetValues 读取指定范围的单元格内容。
func (c *Client) ReadSheetValues(ctx context.Context, raw string, rangeValue string) (*SheetValuesResult, error) {
	target, err := ParseSheetTarget(raw)
	if err != nil {
		return nil, err
	}
	rangeValue = strings.TrimSpace(rangeValue)
	if rangeValue == "" {
		return nil, errors.New("range 不能为空，例如 Sheet1!A1:D20")
	}
	var data map[string]any
	apiPath := "/open-apis/sheets/v2/spreadsheets/" + url.PathEscape(target.SpreadsheetToken) + "/values/" + url.PathEscape(rangeValue)
	if err := c.doJSON(ctx, http.MethodGet, apiPath, nil, &data); err != nil {
		return nil, err
	}
	valueRange, _ := data["valueRange"].(map[string]any)
	if valueRange == nil {
		valueRange, _ = data["value_range"].(map[string]any)
	}
	if valueRange == nil {
		return &SheetValuesResult{
			SpreadsheetToken: target.SpreadsheetToken,
			Range:            rangeValue,
			Values:           [][]any{},
		}, nil
	}
	return &SheetValuesResult{
		SpreadsheetToken: target.SpreadsheetToken,
		Range:            firstNonEmpty(stringField(valueRange, "range"), rangeValue),
		Revision:         intField(valueRange, "revision"),
		Values:           matrixValue(valueRange["values"]),
	}, nil
}

// FindSheet 在指定工作表内查找文本。
func (c *Client) FindSheet(ctx context.Context, raw string, sheetID string, query string, rangeValue string, matchCase bool, matchEntireCell bool, searchByRegex bool, includeFormulas bool) (*SheetFindResult, error) {
	target, err := ParseSheetTarget(raw)
	if err != nil {
		return nil, err
	}
	sheetID = firstNonEmpty(sheetID, target.SheetID)
	if sheetID == "" {
		return nil, errors.New("sheet_id 不能为空；可传 sheet URL 或显式传 sheet_id")
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("query 不能为空")
	}
	if err := c.ensureAccessToken(); err != nil {
		return nil, err
	}
	conditionBuilder := larksheets.NewFindConditionBuilder()
	if strings.TrimSpace(rangeValue) != "" {
		conditionBuilder.Range(strings.TrimSpace(rangeValue))
	}
	if matchCase {
		conditionBuilder.MatchCase(matchCase)
	}
	if matchEntireCell {
		conditionBuilder.MatchEntireCell(matchEntireCell)
	}
	if searchByRegex {
		conditionBuilder.SearchByRegex(searchByRegex)
	}
	if includeFormulas {
		conditionBuilder.IncludeFormulas(includeFormulas)
	}
	body := larksheets.NewFindBuilder().
		FindCondition(conditionBuilder.Build()).
		Find(query).
		Build()
	req := larksheets.NewFindSpreadsheetSheetReqBuilder().
		SpreadsheetToken(target.SpreadsheetToken).
		SheetId(sheetID).
		Find(body).
		Build()
	resp, err := c.sheets.SpreadsheetSheet.Find(ctx, req, c.authOption())
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, sdkCodeError(resp.Code, resp.Msg, resp.RequestId())
	}
	result := &SheetFindResult{
		SpreadsheetToken: target.SpreadsheetToken,
		SheetID:          sheetID,
		Query:            query,
		Range:            strings.TrimSpace(rangeValue),
		FindResult:       map[string]any{},
	}
	if resp.Data == nil || resp.Data.FindResult == nil {
		return result, nil
	}
	result.FindResult, err = sdkObjectToMap(resp.Data.FindResult)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func matrixValue(value any) [][]any {
	if value == nil {
		return [][]any{}
	}
	rawRows, ok := value.([]any)
	if !ok {
		return [][]any{}
	}
	rows := make([][]any, 0, len(rawRows))
	for _, rawRow := range rawRows {
		switch row := rawRow.(type) {
		case []any:
			rows = append(rows, row)
		default:
			rows = append(rows, []any{row})
		}
	}
	return rows
}
