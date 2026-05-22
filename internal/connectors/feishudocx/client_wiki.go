package feishudocx

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ListWikiSpaces 列出当前 token 可访问的知识库空间。
func (c *Client) ListWikiSpaces(ctx context.Context, pageToken string, pageSize int) (*WikiSpaceListResult, error) {
	query := url.Values{}
	query.Set("page_size", fmt.Sprintf("%d", normalizePageSize(pageSize, 50, 50)))
	if strings.TrimSpace(pageToken) != "" {
		query.Set("page_token", strings.TrimSpace(pageToken))
	}
	var data WikiSpaceListResult
	if err := c.doJSON(ctx, http.MethodGet, "/open-apis/wiki/v2/spaces", query, nil, &data); err != nil {
		return nil, err
	}
	if data.Items == nil {
		data.Items = []WikiSpace{}
	}
	return &data, nil
}

// GetWikiSpace 获取单个知识库空间详情。
func (c *Client) GetWikiSpace(ctx context.Context, spaceID string) (*WikiSpace, error) {
	spaceID = strings.TrimSpace(spaceID)
	if spaceID == "" {
		return nil, errors.New("space_id 不能为空")
	}
	var data struct {
		Space WikiSpace `json:"space"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/open-apis/wiki/v2/spaces/"+url.PathEscape(spaceID), nil, nil, &data); err != nil {
		return nil, err
	}
	return &data.Space, nil
}

// ListWikiNodes 分页列出知识库空间内指定父节点的子节点。
func (c *Client) ListWikiNodes(ctx context.Context, spaceID string, parentNodeToken string, pageToken string, pageSize int) (*WikiNodeListResult, error) {
	spaceID = strings.TrimSpace(spaceID)
	if spaceID == "" {
		return nil, errors.New("space_id 不能为空")
	}
	parentNodeToken, err := NormalizeWikiToken(parentNodeToken)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("page_size", fmt.Sprintf("%d", normalizePageSize(pageSize, 50, 50)))
	if parentNodeToken != "" {
		query.Set("parent_node_token", parentNodeToken)
	}
	if strings.TrimSpace(pageToken) != "" {
		query.Set("page_token", strings.TrimSpace(pageToken))
	}
	var data WikiNodeListResult
	path := "/open-apis/wiki/v2/spaces/" + url.PathEscape(spaceID) + "/nodes"
	if err := c.doJSON(ctx, http.MethodGet, path, query, nil, &data); err != nil {
		return nil, err
	}
	for index := range data.Items {
		c.enrichWikiNode(&data.Items[index])
	}
	if data.Items == nil {
		data.Items = []WikiNode{}
	}
	return &data, nil
}

// GetWikiNode 获取 Wiki URL 或 wiki node token 对应的实际对象。
func (c *Client) GetWikiNode(ctx context.Context, wikiToken string) (*WikiNode, error) {
	return c.GetWikiNodeByToken(ctx, wikiToken, "wiki")
}

// GetWikiNodeByToken 获取指定 token 和 obj_type 对应的 Wiki 节点元数据。
func (c *Client) GetWikiNodeByToken(ctx context.Context, token string, objType string) (*WikiNode, error) {
	token, err := NormalizeWikiToken(token)
	if err != nil {
		return nil, err
	}
	if token == "" {
		return nil, errors.New("token 不能为空")
	}
	objType = strings.TrimSpace(objType)
	if objType == "" {
		objType = "wiki"
	}
	query := url.Values{}
	query.Set("token", token)
	query.Set("obj_type", objType)
	var data struct {
		Node WikiNode `json:"node"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/open-apis/wiki/v2/spaces/get_node", query, nil, &data); err != nil {
		return nil, err
	}
	c.enrichWikiNode(&data.Node)
	return &data.Node, nil
}

// NormalizeWikiToken 从 Wiki URL 或纯 token 中提取 node token。
func NormalizeWikiToken(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		if strings.Contains(value, "/") {
			return "", fmt.Errorf("Wiki URL 格式不正确: %s", value)
		}
		return value, nil
	}
	segments := splitPath(parsed.Path)
	for index, segment := range segments {
		if segment == "wiki" && index+1 < len(segments) {
			return strings.TrimSpace(segments[index+1]), nil
		}
	}
	return "", fmt.Errorf("URL 中未找到 Wiki node token: %s", value)
}

func (c *Client) enrichWikiNode(node *WikiNode) {
	if node == nil {
		return
	}
	if node.NodeToken != "" && node.NodeURL == "" {
		node.NodeURL = c.docBaseURL + "/wiki/" + url.PathEscape(node.NodeToken)
	}
	if node.ObjToken == "" || node.DocumentURL != "" {
		return
	}
	switch strings.TrimSpace(node.ObjType) {
	case "doc", "docx":
		node.DocumentURL = c.docBaseURL + "/" + node.ObjType + "/" + url.PathEscape(node.ObjToken)
	case "sheet":
		node.DocumentURL = c.docBaseURL + "/sheets/" + url.PathEscape(node.ObjToken)
	case "bitable":
		node.DocumentURL = c.docBaseURL + "/base/" + url.PathEscape(node.ObjToken)
	}
}

func normalizePageSize(value int, fallback int, max int) int {
	if value <= 0 {
		return fallback
	}
	if value > max {
		return max
	}
	return value
}
