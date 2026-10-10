package server

import (
	"context"
	"strings"
	"time"

	teamhandler "github.com/nexus-research-lab/nexus/internal/handler/team"
	relaycontract "github.com/nexus-research-lab/nexus/internal/relay"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	relaysvc "github.com/nexus-research-lab/nexus/internal/service/relay"
	teamsvc "github.com/nexus-research-lab/nexus/internal/service/team"
	teamstore "github.com/nexus-research-lab/nexus/internal/storage/teamrelay"
)

func (s *Server) mountTeamNodeRoutes() {
	if s.config.MultiplayerDisabled || s.services == nil || s.services.Core == nil || s.services.DB == nil {
		return
	}
	if strings.TrimSpace(s.config.RemoteURL) == "" && strings.TrimSpace(s.config.ControlURL) == "" {
		return
	}
	desktop := strings.EqualFold(strings.TrimSpace(s.config.AppMode), "desktop")
	if !desktop && s.services.Relay == nil {
		return
	}
	// 服务端复用应用级 Relay 客户端；Desktop 的 Relay 由固定远程 Gateway 提供。
	relay := s.services.Relay
	var readRoom func(context.Context, string, string) (relaycontract.RoomDetails, error)
	if desktop {
		client, err := relaysvc.NewClient(s.config.RemoteURL, time.Duration(s.config.RelayRequestTimeoutSeconds)*time.Second)
		if err != nil {
			s.api.BaseLogger().Error("本机节点执行未启用，远程地址配置无效")
		}
		relay = client
	} else {
		readRoom = func(ctx context.Context, _ string, roomID string) (relaycontract.RoomDetails, error) {
			control, ok := s.services.Auth.(*authsvc.ControlAuthority)
			if !ok {
				return relaycontract.RoomDetails{}, teamsvc.ErrNodeUnavailable
			}
			token, err := control.ExchangeRelayUserToken(ctx, authsvc.PrincipalFromContext(ctx))
			if err != nil {
				return relaycontract.RoomDetails{}, err
			}
			return relay.GetRoomWithMembers(ctx, token, roomID)
		}
	}
	service, err := teamsvc.NewNodeService(s.config, teamstore.NewRepository(s.config, s.services.DB), s.services.Core.Agent.ListAgents, readRoom)
	if err != nil {
		s.api.BaseLogger().Error("本机节点授权未启用，服务地址配置无效")
		return
	}
	handler := teamhandler.NewNodeHandlers(s.api, service, s.config.AuthSessionCookieName)
	if relay != nil && s.services.RoomRealtime != nil && s.services.Core.Room != nil {
		s.teamExecutor = teamsvc.NewNodeExecutor(service, relay, s.services.Core.Room, s.services.RoomRealtime, s.services.Workspace, s.api.BaseLogger())
	}
	// 不能挂在 /team 代理下；本地和在线登录各自提供宿主与远程账号证据。
	path := s.prefixPath("/team-node")
	s.router.Get(path, handler.Handle)
	s.router.Post(path, handler.Handle)
	s.router.Delete(path, handler.Handle)
	s.router.Post(path+"/room", handler.HandleRoom)
	s.router.Post(path+"/jobs/recover", handler.HandleRecover)
}

func (s *Server) startTeamExecutor(ctx context.Context) (func(), error) {
	if s.teamExecutor == nil {
		return nil, nil
	}
	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); s.teamExecutor.Run(workerCtx) }()
	return func() { cancel(); <-done }, nil
}
