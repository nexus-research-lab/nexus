package feishudocx

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkwiki "github.com/larksuite/oapi-sdk-go/v3/service/wiki/v2"
)

// WikiSpace 表示飞书知识库空间元数据。
type WikiSpace struct {
	SpaceID     string `json:"space_id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	SpaceType   string `json:"space_type,omitempty"`
	Visibility  string `json:"visibility,omitempty"`
}

// WikiSpaceListResult 表示知识库空间分页列表。
type WikiSpaceListResult struct {
	Items     []WikiSpace `json:"items"`
	PageToken string      `json:"page_token,omitempty"`
	HasMore   bool        `json:"has_more"`
}

// WikiNode 表示 Wiki 节点解析后的对象元数据。
type WikiNode struct {
	SpaceID         string `json:"space_id,omitempty"`
	NodeToken       string `json:"node_token,omitempty"`
	ObjToken        string `json:"obj_token,omitempty"`
	ObjType         string `json:"obj_type,omitempty"`
	ParentNodeToken string `json:"parent_node_token,omitempty"`
	NodeType        string `json:"node_type,omitempty"`
	OriginNodeToken string `json:"origin_node_token,omitempty"`
	OriginSpaceID   string `json:"origin_space_id,omitempty"`
	HasChild        bool   `json:"has_child,omitempty"`
	Title           string `json:"title,omitempty"`
	ObjCreateTime   string `json:"obj_create_time,omitempty"`
	ObjEditTime     string `json:"obj_edit_time,omitempty"`
	NodeCreateTime  string `json:"node_create_time,omitempty"`
	Creator         string `json:"creator,omitempty"`
	Owner           string `json:"owner,omitempty"`
	NodeURL         string `json:"node_url,omitempty"`
	DocumentURL     string `json:"document_url,omitempty"`
}

// WikiNodeListResult 表示知识库节点分页列表。
type WikiNodeListResult struct {
	Items     []WikiNode `json:"items"`
	PageToken string     `json:"page_token,omitempty"`
	HasMore   bool       `json:"has_more"`
}

// ListWikiSpaces 列出当前 token 可访问的知识库空间。
func (c *Client) ListWikiSpaces(ctx context.Context, pageToken string, pageSize int) (*WikiSpaceListResult, error) {
	if err := c.ensureAccessToken(); err != nil {
		return nil, err
	}
	builder := larkwiki.NewListSpaceReqBuilder().
		PageSize(normalizePageSize(pageSize, 50, 50))
	if strings.TrimSpace(pageToken) != "" {
		builder.PageToken(strings.TrimSpace(pageToken))
	}
	resp, err := c.wiki.Space.List(ctx, builder.Build(), c.authOption())
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, sdkCodeError(resp.Code, resp.Msg, resp.RequestId())
	}
	result := &WikiSpaceListResult{Items: []WikiSpace{}}
	if resp.Data == nil {
		return result, nil
	}
	for _, item := range resp.Data.Items {
		result.Items = append(result.Items, sdkWikiSpace(item))
	}
	result.PageToken = larkcore.StringValue(resp.Data.PageToken)
	result.HasMore = larkcore.BoolValue(resp.Data.HasMore)
	return result, nil
}

// GetWikiSpace 获取单个知识库空间详情。
func (c *Client) GetWikiSpace(ctx context.Context, spaceID string) (*WikiSpace, error) {
	spaceID = strings.TrimSpace(spaceID)
	if spaceID == "" {
		return nil, errors.New("space_id 不能为空")
	}
	if err := c.ensureAccessToken(); err != nil {
		return nil, err
	}
	req := larkwiki.NewGetSpaceReqBuilder().
		SpaceId(spaceID).
		Build()
	resp, err := c.wiki.Space.Get(ctx, req, c.authOption())
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, sdkCodeError(resp.Code, resp.Msg, resp.RequestId())
	}
	if resp.Data == nil || resp.Data.Space == nil {
		return nil, errors.New("飞书知识库响应缺少 space")
	}
	space := sdkWikiSpace(resp.Data.Space)
	return &space, nil
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
	if err := c.ensureAccessToken(); err != nil {
		return nil, err
	}
	builder := larkwiki.NewListSpaceNodeReqBuilder().
		SpaceId(spaceID).
		PageSize(normalizePageSize(pageSize, 50, 50))
	if parentNodeToken != "" {
		builder.ParentNodeToken(parentNodeToken)
	}
	if strings.TrimSpace(pageToken) != "" {
		builder.PageToken(strings.TrimSpace(pageToken))
	}
	resp, err := c.wiki.SpaceNode.List(ctx, builder.Build(), c.authOption())
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, sdkCodeError(resp.Code, resp.Msg, resp.RequestId())
	}
	result := &WikiNodeListResult{Items: []WikiNode{}}
	if resp.Data == nil {
		return result, nil
	}
	for _, item := range resp.Data.Items {
		node := sdkWikiNode(item)
		c.enrichWikiNode(&node)
		result.Items = append(result.Items, node)
	}
	result.PageToken = larkcore.StringValue(resp.Data.PageToken)
	result.HasMore = larkcore.BoolValue(resp.Data.HasMore)
	return result, nil
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
	if err := c.ensureAccessToken(); err != nil {
		return nil, err
	}
	req := larkwiki.NewGetNodeSpaceReqBuilder().
		Token(token).
		ObjType(objType).
		Build()
	resp, err := c.wiki.Space.GetNode(ctx, req, c.authOption())
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, sdkCodeError(resp.Code, resp.Msg, resp.RequestId())
	}
	if resp.Data == nil || resp.Data.Node == nil {
		return nil, errors.New("飞书知识库响应缺少 node")
	}
	node := sdkWikiNode(resp.Data.Node)
	c.enrichWikiNode(&node)
	return &node, nil
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
	return min(value, max)
}

func sdkWikiSpace(space *larkwiki.Space) WikiSpace {
	if space == nil {
		return WikiSpace{}
	}
	return WikiSpace{
		SpaceID:     larkcore.StringValue(space.SpaceId),
		Name:        larkcore.StringValue(space.Name),
		Description: larkcore.StringValue(space.Description),
		SpaceType:   larkcore.StringValue(space.SpaceType),
		Visibility:  larkcore.StringValue(space.Visibility),
	}
}

func sdkWikiNode(node *larkwiki.Node) WikiNode {
	if node == nil {
		return WikiNode{}
	}
	return WikiNode{
		SpaceID:         larkcore.StringValue(node.SpaceId),
		NodeToken:       larkcore.StringValue(node.NodeToken),
		ObjToken:        larkcore.StringValue(node.ObjToken),
		ObjType:         larkcore.StringValue(node.ObjType),
		ParentNodeToken: larkcore.StringValue(node.ParentNodeToken),
		NodeType:        larkcore.StringValue(node.NodeType),
		OriginNodeToken: larkcore.StringValue(node.OriginNodeToken),
		OriginSpaceID:   larkcore.StringValue(node.OriginSpaceId),
		HasChild:        larkcore.BoolValue(node.HasChild),
		Title:           larkcore.StringValue(node.Title),
		ObjCreateTime:   larkcore.StringValue(node.ObjCreateTime),
		ObjEditTime:     larkcore.StringValue(node.ObjEditTime),
		NodeCreateTime:  larkcore.StringValue(node.NodeCreateTime),
		Creator:         larkcore.StringValue(node.Creator),
		Owner:           larkcore.StringValue(node.Owner),
	}
}
