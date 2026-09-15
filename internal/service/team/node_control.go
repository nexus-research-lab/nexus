package team

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

var (
	ErrNodeLogin       = errors.New("节点授权需要有效远程登录")
	ErrNodeInput       = errors.New("节点授权参数无效")
	ErrNodeUnavailable = errors.New("节点授权服务暂时不可用")
)

type nodeRemoteError struct{ status int }

func (e *nodeRemoteError) Error() string { return "Control 节点请求失败" }

// callControl 只向固定服务发送单个 Session Cookie，不转发宿主 header，也不跟随重定向。
func (s *NodeService) callControl(ctx context.Context, cookie, method, path string, input, output any) error {
	return s.controlRequest(ctx, cookie, "", method, path, input, output)
}

func (s *NodeService) controlRequest(ctx context.Context, cookie, credential, method, path string, input, output any) error {
	data, err := json.Marshal(input)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, method, s.remoteURL+"/auth/v1"+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	if cookie != "" {
		request.AddCookie(&http.Cookie{Name: s.cookieName, Value: cookie})
	}
	if credential != "" {
		request.Header.Set("Authorization", "Bearer "+credential)
	}
	request.Header.Set("Origin", s.origin)
	request.Header.Set("Content-Type", "application/json")
	response, err := s.httpClient.Do(request)
	if err != nil {
		return ErrNodeUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized {
		return ErrNodeLogin
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &nodeRemoteError{response.StatusCode}
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&envelope); err != nil {
		return ErrNodeUnavailable
	}
	if output != nil {
		if err = json.Unmarshal(envelope.Data, output); err != nil {
			return ErrNodeUnavailable
		}
	}
	return nil
}

type nodeToken struct {
	Token     string      `json:"token"`
	ExpiresAt time.Time   `json:"expires_at"`
	Agents    []nodeAgent `json:"agents"`
}

type nodeIdentity struct {
	Authenticated  bool   `json:"authenticated"`
	UserID         string `json:"user_id"`
	OrganizationID string `json:"organization_id"`
}

type nodeAgent struct {
	AgentID       string `json:"agent_id"`
	SourceAgentID string `json:"source_agent_id"`
	Name          string `json:"name"`
}

type nodeStatus struct {
	NodeID    string     `json:"node_id"`
	Name      string     `json:"name"`
	AgentIDs  []string   `json:"agent_ids"`
	RevokedAt *time.Time `json:"revoked_at"`
}
