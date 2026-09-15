package team

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	teamstore "github.com/nexus-research-lab/nexus/internal/storage/teamrelay"
)

// NodeService 只管理本机执行授权，不把授权成功伪装成 runtime 已连接。
type NodeService struct {
	remoteURL, origin, cookieName string
	httpClient                    *http.Client
	store                         *teamstore.Repository
	keys                          *credentials.Keyring
	listAgents                    func(context.Context) ([]protocol.Agent, error)
	executor                      *NodeExecutor
}

type NodeView struct {
	NodeID             string          `json:"node_id,omitempty"`
	State              string          `json:"state"`
	Name               string          `json:"name,omitempty"`
	AgentIDs           []string        `json:"agent_ids"`
	Candidates         []NodeCandidate `json:"candidates"`
	ExecutionAvailable bool            `json:"execution_available"`
	ExecutionEnabled   bool            `json:"execution_enabled"`
	Jobs               []NodeJobView   `json:"jobs"`
}
type NodeCandidate struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type NodeConnectInput struct {
	Name            string   `json:"name"`
	AgentIDs        []string `json:"agent_ids"`
	EnableExecution bool     `json:"enable_execution"`
}

func NewNodeService(cfg config.Config, store *teamstore.Repository, listAgents func(context.Context) ([]protocol.Agent, error)) (*NodeService, error) {
	remote := strings.TrimRight(strings.TrimSpace(cfg.RemoteURL), "/")
	if !strings.EqualFold(strings.TrimSpace(cfg.AppMode), "desktop") {
		remote = strings.TrimRight(strings.TrimSpace(cfg.ControlURL), "/")
	}
	parsed, err := url.Parse(remote)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, ErrNodeInput
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	keys, _ := credentials.NewKeyring(cfg.ConnectorCredentialsKey, cfg.ConnectorCredentialsLegacyKeys)
	return &NodeService{remoteURL: remote, origin: parsed.Scheme + "://" + parsed.Host, cookieName: cfg.AuthSessionCookieName,
		httpClient: &http.Client{Timeout: 5 * time.Second, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		store:      store, keys: keys, listAgents: listAgents}, nil
}

func nodeDigest(value any) string {
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func (s *NodeService) scope(ctx context.Context, cookie string) (string, string, error) {
	owner, ok := authctx.CurrentUserID(ctx)
	if !ok || cookie == "" {
		return "", "", ErrNodeLogin
	}
	var identity nodeIdentity
	if err := s.callControl(ctx, cookie, http.MethodGet, "/status", nil, &identity); err != nil {
		return "", "", err
	}
	if !identity.Authenticated || identity.UserID == "" || identity.OrganizationID == "" {
		return "", "", ErrNodeLogin
	}
	return nodeDigest([]string{owner, s.remoteURL, identity.UserID, identity.OrganizationID}), owner, nil
}

func (s *NodeService) candidates(ctx context.Context, cookie string) ([]NodeCandidate, error) {
	local, err := s.listAgents(ctx)
	if err != nil {
		return nil, err
	}
	var online []nodeAgent
	if err = s.callControl(ctx, cookie, http.MethodGet, "/agents", nil, &online); err != nil {
		return nil, err
	}
	owned := make(map[string]string, len(local))
	for _, agent := range local {
		owned[agent.AgentID] = agent.Name
	}
	result := make([]NodeCandidate, 0)
	for _, agent := range online {
		if name, ok := owned[agent.SourceAgentID]; ok {
			result = append(result, NodeCandidate{ID: agent.AgentID, Name: name})
		}
	}
	return result, nil
}

func (s *NodeService) View(ctx context.Context, cookie string) (NodeView, error) {
	scope, owner, err := s.scope(ctx, cookie)
	if err != nil {
		return NodeView{}, err
	}
	record, err := s.store.NodeGrant(ctx, scope, owner)
	if err != nil {
		return NodeView{}, err
	}
	if err = s.reconcile(ctx, cookie, record); err != nil {
		return NodeView{}, err
	}
	candidates, err := s.candidates(ctx, cookie)
	if err != nil {
		return NodeView{}, err
	}
	view := NodeView{State: "disconnected", AgentIDs: []string{}, Candidates: candidates}
	view.ExecutionAvailable = s.executor != nil && s.executor.ready.Load()
	view.Jobs = []NodeJobView{}
	if record != nil {
		view.NodeID, view.State, view.Name, view.AgentIDs = record.NodeID, record.State, record.Name, record.AgentIDs
		view.ExecutionEnabled = record.ExecutionEnabled && record.State == "authorized"
		jobs, err := s.store.NodeJobs(ctx, owner, scope)
		if err != nil {
			return NodeView{}, err
		}
		for _, job := range jobs {
			state := job.State
			if state == "running" && (s.executor == nil || !s.executor.running(job.ID)) {
				state = "review_required"
			}
			view.Jobs = append(view.Jobs, NodeJobView{ID: job.ID, AgentID: job.AgentID, State: state, RoomID: job.RoomID, ConversationID: job.ConversationID})
		}
	}
	return view, nil
}

func (s *NodeService) Connect(ctx context.Context, cookie string, input NodeConnectInput) error {
	if input.EnableExecution && (s.executor == nil || !s.executor.ready.Load()) {
		return ErrNodeUnavailable
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len(input.Name) > 128 || len(input.AgentIDs) == 0 || len(input.AgentIDs) > 32 {
		return ErrNodeInput
	}
	input.AgentIDs = slices.Clone(input.AgentIDs)
	slices.Sort(input.AgentIDs)
	input.AgentIDs = slices.Compact(input.AgentIDs)
	scope, owner, err := s.scope(ctx, cookie)
	if err != nil {
		return err
	}
	candidates, err := s.candidates(ctx, cookie)
	if err != nil {
		return err
	}
	for _, id := range input.AgentIDs {
		if !slices.ContainsFunc(candidates, func(candidate NodeCandidate) bool { return candidate.ID == id }) {
			return ErrNodeInput
		}
	}
	if s.keys == nil {
		return credentials.ErrKeyUnavailable
	}
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return err
	}
	credential := base64.RawURLEncoding.EncodeToString(secret)
	encrypted, err := s.keys.EncryptEnvelope([]byte(credential))
	if err != nil {
		return err
	}
	record, err := s.store.PrepareNodeGrant(ctx, teamstore.NodeGrant{Scope: scope, OwnerUserID: owner, NodeID: "node_" + nodeDigest(credential)[:32], State: "pending", Name: input.Name, CookieHash: nodeDigest(cookie), CredentialEncrypted: encrypted, AgentIDs: input.AgentIDs, RemoteURL: s.remoteURL, ExecutionEnabled: input.EnableExecution})
	if err != nil {
		return err
	}
	if record == nil || record.CookieHash != nodeDigest(cookie) || record.Name != input.Name || !slices.Equal(record.AgentIDs, input.AgentIDs) || (record.State != "pending" && record.State != "authorized") {
		return teamstore.ErrNodeConflict
	}
	plain, err := s.keys.DecryptEnvelope(record.CredentialEncrypted)
	if err != nil {
		return err
	}
	var result nodeStatus
	err = s.callControl(ctx, cookie, http.MethodPost, "/nodes", map[string]any{"node_id": record.NodeID, "name": record.Name, "credential": string(plain), "agent_ids": record.AgentIDs}, &result)
	if err != nil {
		return err
	}
	if result.NodeID != record.NodeID || result.RevokedAt != nil || !slices.Equal(result.AgentIDs, record.AgentIDs) {
		return ErrNodeUnavailable
	}
	record.ExecutionEnabled, record.RemoteURL = input.EnableExecution, s.remoteURL
	if record.State == "authorized" {
		return s.store.SetNodeState(ctx, *record, "authorized", "authorized")
	}
	return s.store.SetNodeState(ctx, *record, "pending", "authorized")
}

type NodeJobView struct {
	ID             string `json:"id"`
	AgentID        string `json:"agent_id"`
	State          string `json:"state"`
	RoomID         string `json:"room_id,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
}

// reconcile 只用精确节点回执推进状态；未知注册的 404 不能证明迟到请求不会提交。
func (s *NodeService) reconcile(ctx context.Context, cookie string, record *teamstore.NodeGrant) error {
	if record == nil || record.State == "revoked" {
		return nil
	}
	var result nodeStatus
	err := s.callControl(ctx, cookie, http.MethodGet, "/nodes/"+url.PathEscape(record.NodeID), nil, &result)
	var remote *nodeRemoteError
	missing := errors.As(err, &remote) && remote.status == http.StatusNotFound
	if err != nil && !missing {
		return err
	}
	if missing {
		return nil
	}
	if result.NodeID != record.NodeID || (result.RevokedAt == nil && (result.Name != record.Name || !slices.Equal(result.AgentIDs, record.AgentIDs))) {
		return ErrNodeUnavailable
	}
	next := record.State
	if result.RevokedAt != nil {
		next = "revoked"
	} else if record.State == "pending" {
		next = "authorized"
	}
	if next == record.State {
		return nil
	}
	if err = s.store.SetNodeState(ctx, *record, record.State, next); err != nil {
		return err
	}
	record.State = next
	return nil
}

func (s *NodeService) Revoke(ctx context.Context, cookie string) error {
	scope, owner, err := s.scope(ctx, cookie)
	if err != nil {
		return err
	}
	record, err := s.store.NodeGrant(ctx, scope, owner)
	if err != nil || record == nil {
		return err
	}
	if err = s.reconcile(ctx, cookie, record); err != nil {
		return err
	}
	if record.State == "revoked" {
		return nil
	}
	if record.State != "revoking" {
		if err = s.store.SetNodeState(ctx, *record, record.State, "revoking"); err != nil {
			return err
		}
	}
	err = s.callControl(ctx, cookie, http.MethodDelete, "/nodes/"+url.PathEscape(record.NodeID), nil, nil)
	if err != nil {
		return err
	}
	return s.store.SetNodeState(ctx, *record, "revoking", "revoked")
}
