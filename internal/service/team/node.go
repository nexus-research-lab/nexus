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
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	relaycontract "github.com/nexus-research-lab/nexus/internal/relay"
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
	readRoom                      func(context.Context, string, string) (relaycontract.RoomDetails, error)
	provisionMu                   sync.Mutex
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

func NewNodeService(cfg config.Config, store *teamstore.Repository, listAgents func(context.Context) ([]protocol.Agent, error), readRoom func(context.Context, string, string) (relaycontract.RoomDetails, error)) (*NodeService, error) {
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
		store:      store, keys: keys, listAgents: listAgents, readRoom: readRoom}, nil
}

// NodeRoomBinding 与任务历史无关；本人入群同时授权执行，仍只由显式投递启动 Agent。
type NodeRoomBinding struct {
	AgentID        string `json:"agent_id"`
	LocalAgentID   string `json:"local_agent_id"`
	RoomID         string `json:"room_id"`
	ConversationID string `json:"conversation_id"`
}

func (s *NodeService) PrepareRoom(ctx context.Context, cookie, roomID string) ([]NodeRoomBinding, error) {
	if roomID == "" || len(roomID) > 128 || strings.ContainsAny(roomID, "/?#") {
		return nil, ErrNodeInput
	}
	if s.executor == nil {
		return nil, ErrNodeUnavailable
	}
	scope, _, err := s.scope(ctx, cookie)
	if err != nil {
		return nil, err
	}
	var details relaycontract.RoomDetails
	if s.readRoom != nil {
		details, err = s.readRoom(ctx, cookie, roomID)
	} else {
		// Desktop 只向固定远程 Gateway 发送 Cookie，不从请求体接收服务地址。
		err = s.remoteRequest(ctx, cookie, "", http.MethodGet, "/nexus/v1/team/rooms/"+url.PathEscape(roomID), nil, &details)
	}
	if err != nil {
		return nil, err
	}
	var online []nodeAgent
	if err = s.callControl(ctx, cookie, http.MethodGet, "/agents", nil, &online); err != nil {
		return nil, err
	}
	local, err := s.listAgents(ctx)
	if err != nil {
		return nil, err
	}
	bindings := make([]NodeRoomBinding, 0)
	var executable []string
	for _, agent := range online {
		if !slices.ContainsFunc(local, func(value protocol.Agent) bool {
			return value.AgentID == agent.SourceAgentID && value.Status == "active" && !value.IsMain
		}) {
			continue
		}
		if !slices.ContainsFunc(details.Members, func(member relaycontract.RoomMember) bool {
			return member.Type == "agent" && member.ID == agent.AgentID && member.State == "active"
		}) {
			continue
		}
		room, err := s.executor.prepare(ctx, scope+":"+roomID, agent.SourceAgentID)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, NodeRoomBinding{AgentID: agent.AgentID, LocalAgentID: agent.SourceAgentID, RoomID: room.Room.ID, ConversationID: room.Conversation.ID})
		if slices.ContainsFunc(details.Members, func(member relaycontract.RoomMember) bool {
			return member.Type == "agent" && member.ID == agent.AgentID && member.State == "active" && !member.AgentPaused
		}) {
			executable = append(executable, agent.AgentID)
		}
	}
	if len(executable) > 0 {
		if err := s.ensureRoomExecution(ctx, cookie, executable); err != nil {
			return nil, err
		}
	}
	return bindings, nil
}

// 只消费已由 PrepareRoom 校验的本人入群成员；凭据未知写入复用原意图，失效恢复仍须有效真人登录。
func (s *NodeService) ensureRoomExecution(ctx context.Context, cookie string, agentIDs []string) error {
	s.provisionMu.Lock()
	defer s.provisionMu.Unlock()
	scope, owner, err := s.scope(ctx, cookie)
	if err != nil {
		return err
	}
	record, err := s.store.NodeGrant(ctx, scope, owner)
	if err != nil {
		return err
	}
	if record == nil || record.State == "revoked" {
		return s.Connect(ctx, cookie, NodeConnectInput{Name: "Nexus", AgentIDs: agentIDs, EnableExecution: true})
	}
	// 未确认登记先重放原请求，不换凭据、不以 404 推断未提交。
	if record.State == "pending" && record.CookieHash == nodeDigest(cookie) {
		if err = s.Connect(ctx, cookie, NodeConnectInput{Name: record.Name, AgentIDs: record.AgentIDs, EnableExecution: true}); err != nil {
			return err
		}
		record, err = s.store.NodeGrant(ctx, scope, owner)
		if err != nil {
			return err
		}
		if record == nil {
			return ErrNodeUnavailable
		}
	}
	ids := slices.Clone(agentIDs)
	candidates, err := s.candidates(ctx, cookie)
	if err != nil {
		return err
	}
	for _, id := range record.AgentIDs {
		if slices.ContainsFunc(candidates, func(candidate NodeCandidate) bool { return candidate.ID == id }) {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	if record.State == "authorized" && record.CookieHash == nodeDigest(cookie) && slices.Equal(record.AgentIDs, ids) {
		wasEnabled := record.ExecutionEnabled
		record.ExecutionEnabled = true
		_, err = s.executor.machineToken(ctx, *record)
		if err == nil {
			if !wasEnabled {
				return s.setNodeState(ctx, *record, "authorized", "authorized")
			}
			return nil
		}
		if !errors.Is(err, ErrNodeLogin) {
			return err
		}
	}
	// 变更设备范围前先等待现有任务收尾，不能因新增成员打断其他群的执行。
	local, err := s.listAgents(ctx)
	if err != nil {
		return err
	}
	for _, agent := range local {
		job, err := s.store.ActiveNodeJob(ctx, owner, agent.AgentID)
		if err != nil {
			return err
		}
		if job != nil && job.NodeID == record.NodeID {
			return ErrNodeUnavailable
		}
	}
	if err = s.Revoke(ctx, cookie); err != nil {
		return err
	}
	return s.Connect(ctx, cookie, NodeConnectInput{Name: record.Name, AgentIDs: ids, EnableExecution: true})
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

type NodeJobQuery struct {
	RoomID     string
	MessageIDs []string
	JobID      string
}

func (s *NodeService) View(ctx context.Context, cookie string, queries ...NodeJobQuery) (NodeView, error) {
	if len(queries) > 1 {
		return NodeView{}, ErrNodeInput
	}
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
		var jobs []teamstore.NodeJob
		if len(queries) == 1 {
			query := queries[0]
			if query.RoomID == "" || len(query.MessageIDs) > 100 || len(query.RoomID) > 256 || len(query.JobID) > 256 {
				return NodeView{}, ErrNodeInput
			}
			for _, id := range query.MessageIDs {
				if len(id) > 256 || id == "" {
					return NodeView{}, ErrNodeInput
				}
			}
			jobs, err = s.store.NodeMessageJobs(ctx, owner, scope, query.RoomID, query.MessageIDs, query.JobID)
		} else {
			jobs, err = s.store.NodeJobs(ctx, owner, scope)
		}
		if err != nil {
			return NodeView{}, err
		}
		for _, job := range jobs {
			state := job.State
			if state == "running" && (s.executor == nil || !s.executor.running(job.ID)) {
				state = "review_required"
			}
			item := NodeJobView{ID: job.ID, AgentID: job.AgentID, State: state, RoomID: job.RoomID, ConversationID: job.ConversationID, LocalAgentID: job.LocalAgentID, RoundID: job.RoundID}
			if job.Delivery != nil {
				item.SourceRoomID = job.Delivery.RoomID
				item.SourceMessageID = job.Delivery.MessageID
				item.DeliveryID = job.Delivery.ID
			}
			view.Jobs = append(view.Jobs, item)
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
		return s.setNodeState(ctx, *record, "authorized", "authorized")
	}
	return s.setNodeState(ctx, *record, "pending", "authorized")
}

type NodeJobView struct {
	LocalAgentID    string `json:"local_agent_id,omitempty"`
	RoundID         string `json:"round_id,omitempty"`
	SourceRoomID    string `json:"source_room_id,omitempty"`
	SourceMessageID string `json:"source_message_id,omitempty"`
	DeliveryID      string `json:"delivery_id,omitempty"`
	ID              string `json:"id"`
	AgentID         string `json:"agent_id"`
	State           string `json:"state"`
	RoomID          string `json:"room_id,omitempty"`
	ConversationID  string `json:"conversation_id,omitempty"`
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
	if err = s.setNodeState(ctx, *record, record.State, next); err != nil {
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
		if err = s.setNodeState(ctx, *record, record.State, "revoking"); err != nil {
			return err
		}
	}
	err = s.callControl(ctx, cookie, http.MethodDelete, "/nodes/"+url.PathEscape(record.NodeID), nil, nil)
	if err != nil {
		return err
	}
	return s.setNodeState(ctx, *record, "revoking", "revoked")
}

func (s *NodeService) setNodeState(ctx context.Context, record teamstore.NodeGrant, from, to string) error {
	if err := s.store.SetNodeState(ctx, record, from, to); err != nil {
		return err
	}
	if s.executor != nil {
		s.executor.loop.Notify()
	}
	return nil
}
