// INPUT: 已验证 owner/deployment、短期 Relay 令牌与消息/同步请求。
// OUTPUT: 已完成本地投影的读取结果，或保持远端已提交事实的消息回执。
// POS: Team 同步流程的唯一业务协调边界；HTTP 不裁决投影成功语义。
package team

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	relaycontract "github.com/nexus-research-lab/nexus/internal/relay"
)

// ErrProjection 表示远端结果尚未完成本地投影，调用方不能推进读取游标。
var ErrProjection = errors.New("Team 本地投影失败")

// Access 仅由可信入口在核验 Principal 并交换令牌后构造。
type Access struct {
	OwnerUserID  string
	DeploymentID string
	Token        string
}

// RelayClient 是同步流程所需的远端操作端口。
type RelayClient interface {
	ListRooms(context.Context, string) (relaycontract.RoomList, error)
	CreateRoom(context.Context, string, string, relaycontract.CreateRoomInput) (relaycontract.RoomView, error)
	PostMessage(
		context.Context,
		string,
		string,
		string,
		relaycontract.CreateMessageInput,
	) (relaycontract.MessageCommit, error)
	Snapshot(
		context.Context,
		string,
		string,
		relaycontract.SnapshotOptions,
	) (relaycontract.Snapshot, error)
	Difference(
		context.Context,
		string,
		string,
		relaycontract.DifferenceOptions,
	) (relaycontract.Difference, error)
}

// Projector 负责将远端权威结果写入本地读模型。
type Projector interface {
	ProjectRoom(context.Context, string, string, relaycontract.RoomView) error
	ProjectCommit(context.Context, string, relaycontract.MessageCommit) error
	ProjectSnapshot(context.Context, string, relaycontract.Snapshot) error
	ProjectDifference(context.Context, string, relaycontract.Difference) error
}

// Service 协调 Relay 权威结果与 Nexus 本地读模型。
type Service struct {
	relay  RelayClient
	local  Projector
	logger *slog.Logger
}

// New 绑定客户端与本地投影，不启动后台消费者。
func New(relay RelayClient, local Projector, logger *slog.Logger) *Service {
	if logger == nil {
		logger = logx.NewDiscardLogger()
	}
	return &Service{relay: relay, local: local, logger: logger}
}

func (s *Service) project(apply func() error) error {
	if s.local == nil {
		return nil
	}
	if err := apply(); err != nil {
		return fmt.Errorf("%w: %w", ErrProjection, err)
	}
	return nil
}

// ListRooms 返回已建立本地投影的在线目录。
func (s *Service) ListRooms(ctx context.Context, access Access) (relaycontract.RoomList, error) {
	result, err := s.relay.ListRooms(ctx, access.Token)
	if err != nil {
		return result, err
	}
	err = s.project(func() error {
		for _, room := range result.Rooms {
			if err := s.local.ProjectRoom(ctx, access.OwnerUserID, access.DeploymentID, room); err != nil {
				return err
			}
		}
		return nil
	})
	return result, err
}

// CreateRoom 创建远端 Room 并确认本地投影。
func (s *Service) CreateRoom(ctx context.Context, access Access, key string, input relaycontract.CreateRoomInput) (relaycontract.RoomView, error) {
	result, err := s.relay.CreateRoom(ctx, access.Token, key, input)
	if err != nil {
		return result, err
	}
	return result, s.project(func() error { return s.local.ProjectRoom(ctx, access.OwnerUserID, access.DeploymentID, result) })
}

// PostMessage 保留远端已提交事实，本地失败由同步恢复。
func (s *Service) PostMessage(ctx context.Context, access Access, conversationID, key string, input relaycontract.CreateMessageInput) (relaycontract.MessageCommit, error) {
	result, err := s.relay.PostMessage(ctx, access.Token, conversationID, key, input)
	if err != nil {
		return result, err
	}
	if err = s.project(func() error { return s.local.ProjectCommit(ctx, access.OwnerUserID, result) }); err != nil {
		// 远端提交不可撤销，保留成功回执；恢复沿原游标读取 Difference，不能重发消息。
		s.logger.Error("Team 本地投影失败", "owner_user_id", access.OwnerUserID, "err", err)
	}
	return result, nil
}

// Snapshot 只在快照页投影完成后返回成功。
func (s *Service) Snapshot(ctx context.Context, access Access, conversationID string, options relaycontract.SnapshotOptions) (relaycontract.Snapshot, error) {
	result, err := s.relay.Snapshot(ctx, access.Token, conversationID, options)
	if err != nil {
		return result, err
	}
	return result, s.project(func() error { return s.local.ProjectSnapshot(ctx, access.OwnerUserID, result) })
}

// Difference 只在增量投影完成后允许调用方推进游标。
func (s *Service) Difference(ctx context.Context, access Access, streamID string, options relaycontract.DifferenceOptions) (relaycontract.Difference, error) {
	result, err := s.relay.Difference(ctx, access.Token, streamID, options)
	if err != nil {
		return result, err
	}
	return result, s.project(func() error { return s.local.ProjectDifference(ctx, access.OwnerUserID, result) })
}
