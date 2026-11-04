// INPUT: 独立 CLI 服务连接的建群/改名与真实 WebSocket 全局订阅。
// OUTPUT: 未订阅新 Room 的客户端收到目录通知后可读取新群和新名称。
// POS: 跨进程写入到页面 bootstrap 的应用链路回归。
package server

import (
	"context"
	"encoding/json"
	launchersvc "github.com/nexus-research-lab/nexus/internal/service/launcher"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/nexus-research-lab/nexus/internal/app"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestExternalRoomChangeReachesDirectorySubscriber(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	server, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	handlertest.CloseServer(t, server)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stop, err := server.startDirectoryWatch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()
	connection, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(httpServer.URL, "http")+cfg.APIPrefix+"/chat/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.CloseNow()
	if err := wsjson.Write(ctx, connection, map[string]any{"type": "subscribe_app_events"}); err != nil {
		t.Fatal(err)
	}
	var subscribed protocol.EventMessage
	if err := wsjson.Read(ctx, connection, &subscribed); err != nil {
		t.Fatal(err)
	}
	if subscribed.EventType != protocol.EventTypeDirectoryChanged {
		t.Fatalf("missing subscription reconciliation: %+v", subscribed)
	}
	// 与 nexusctl 相同的服务装配，使用独立 DB 连接且不注入 HTTP notifier。
	agents, externalDB, err := app.NewAgentService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer externalDB.Close()
	rooms := app.NewRoomServiceWithDB(cfg, externalDB, agents)
	member, err := agents.CreateAgent(ctx, protocol.CreateRequest{Name: "Directory member"})
	if err != nil {
		t.Fatal(err)
	}
	created, err := rooms.CreateRoom(ctx, protocol.CreateRoomRequest{Name: "External group", AgentIDs: []string{member.AgentID}})
	if err != nil {
		t.Fatal(err)
	}
	awaitName := func(name string) {
		t.Helper()
		for {
			var event protocol.EventMessage
			if err := wsjson.Read(ctx, connection, &event); err != nil {
				t.Fatal(err)
			}
			if event.EventType != protocol.EventTypeDirectoryChanged {
				continue
			}
			recorder := httptest.NewRecorder()
			server.Router().ServeHTTP(recorder, httptest.NewRequest("GET", cfg.APIPrefix+"/launcher/bootstrap", nil))
			var payload struct {
				Data launchersvc.BootstrapResponse `json:"data"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if recorder.Code != 200 {
				t.Fatalf("bootstrap status = %d", recorder.Code)
			}
			for _, room := range payload.Data.Rooms {
				if room.ID != created.Room.ID || room.Name != name {
					continue
				}
				for _, conversation := range payload.Data.Conversations {
					if conversation.RoomID == room.ID && conversation.ConversationID == created.Conversation.ID {
						return
					}
				}
				t.Fatal("new Room is missing the conversation required by the sidebar")
			}
		}
	}
	awaitName("External group")
	renamed := "Renamed external group"
	if _, err := rooms.UpdateRoom(ctx, created.Room.ID, protocol.UpdateRoomRequest{Name: &renamed}); err != nil {
		t.Fatal(err)
	}
	awaitName(renamed)
}
