package feishudocx

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

// SearchResult 表示飞书云文档全文搜索结果。
type SearchResult struct {
	Query     string         `json:"query"`
	Offset    int            `json:"offset"`
	Count     int            `json:"count"`
	DocsTypes []string       `json:"docs_types,omitempty"`
	OwnerIDs  []string       `json:"owner_ids,omitempty"`
	ChatIDs   []string       `json:"chat_ids,omitempty"`
	Data      map[string]any `json:"data"`
}

// SearchDocuments 在飞书云文档里全文搜索文档。
func (c *Client) SearchDocuments(ctx context.Context, query string, docsTypes []string, ownerIDs []string, chatIDs []string, offset int, count int) (*SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("query 不能为空")
	}
	if offset < 0 {
		offset = 0
	}
	if count <= 0 {
		count = 10
	}
	if count > 50 {
		count = 50
	}
	if offset >= 199 {
		return nil, errors.New("offset 需要小于 199")
	}
	if offset+count >= 200 {
		count = 199 - offset
	}
	body := map[string]any{
		"search_key": query,
		"count":      count,
		"offset":     offset,
	}
	docsTypes = trimStrings(docsTypes)
	ownerIDs = trimStrings(ownerIDs)
	chatIDs = trimStrings(chatIDs)
	if len(docsTypes) > 0 {
		body["docs_types"] = docsTypes
	}
	if len(ownerIDs) > 0 {
		body["owner_ids"] = ownerIDs
	}
	if len(chatIDs) > 0 {
		body["chat_ids"] = chatIDs
	}
	var data map[string]any
	if err := c.doJSON(ctx, http.MethodPost, "/open-apis/suite/docs-api/search/object", body, &data); err != nil {
		return nil, err
	}
	if data == nil {
		data = map[string]any{}
	}
	return &SearchResult{
		Query:     query,
		Offset:    offset,
		Count:     count,
		DocsTypes: docsTypes,
		OwnerIDs:  ownerIDs,
		ChatIDs:   chatIDs,
		Data:      data,
	}, nil
}

func trimStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
