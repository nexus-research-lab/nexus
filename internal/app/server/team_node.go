package server

import (
	"context"
	relaycontract "github.com/nexus-research-lab/nexus/internal/relay"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	"strings"

	teamhandler "github.com/nexus-research-lab/nexus/internal/handler/team"
	relaysvc "github.com/nexus-research-lab/nexus/internal/service/relay"
	teamsvc "github.com/nexus-research-lab/nexus/internal/service/team"
	teamstore "github.com/nexus-research-lab/nexus/internal/storage/teamrelay"
)

func (s *Server) mountTeamNodeRoutes() {
	if s.services == nil || s.services.Core == nil || s.services.DB == nil {
		return
	}
	if strings.TrimSpace(s.config.RemoteURL) == "" && strings.TrimSpace(s.config.ControlURL) == "" {
		return
	}
	var readRoom func(context.Context, string, string) (relaycontract.RoomDetails, error)
	if !strings.EqualFold(strings.TrimSpace(s.config.AppMode), "desktop") {
		readRoom = func(ctx context.Context, _ string, roomID string) (relaycontract.RoomDetails, error) {
			control, ok := s.services.Auth.(*authsvc.ControlAuthority)
			if !ok || s.services.Relay == nil {
				return relaycontract.RoomDetails{}, teamsvc.ErrNodeUnavailable
			}
			token, err := control.ExchangeRelayUserToken(ctx, authsvc.PrincipalFromContext(ctx))
			if err != nil {
				return relaycontract.RoomDetails{}, err
			}
			return s.services.Relay.GetRoom(ctx, token, roomID)
		}
	}
	service, err := teamsvc.NewNodeService(s.config, teamstore.NewRepository(s.config, s.services.DB), s.services.Core.Agent.ListAgents, readRoom)
	if err != nil {
		s.api.BaseLogger().Error("本机节点授权未启用，服务地址配置无效")
		return
	}
	handler := teamhandler.NewNodeHandlers(s.api, service, s.config.AuthSessionCookieName)
	relayURL := s.config.RelayURL
	if strings.EqualFold(strings.TrimSpace(s.config.AppMode), "desktop") {
		relayURL = s.config.RemoteURL
	}
	if relayURL != "" && s.services.RoomRealtime != nil && s.services.Core.Room != nil {
		client, err := relaysvc.NewClient(relayURL, 0)
		if err == nil {
			s.teamExecutor = teamsvc.NewNodeExecutor(service, client, s.services.Core.Room, s.services.RoomRealtime, s.api.BaseLogger())
		}
	}
	// 不能挂在 /team 代理下；本地和在线登录各自提供宿主与远程账号证据。
	path := s.prefixPath("/team-node")
	s.router.Get(path, handler.Handle)
	s.router.Post(path, handler.Handle)
	s.router.Delete(path, handler.Handle)
	s.router.Post(path+"/room", handler.HandleRoom)
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
