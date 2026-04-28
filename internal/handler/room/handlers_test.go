package room_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
)

func TestHandleEnsureDirectRoomAllowsMainAgent(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/nexus/v1/rooms/dm/"+cfg.DefaultAgentID, nil)
	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("主智能体直聊状态码不正确: got=%d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Data struct {
			Room struct {
				RoomType string `json:"room_type"`
			} `json:"room"`
		} `json:"data"`
	}
	if err = json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("解析主智能体直聊响应失败: %v", err)
	}
	if payload.Data.Room.RoomType != "dm" {
		t.Fatalf("主智能体直聊 room_type 不正确: got=%s want=dm", payload.Data.Room.RoomType)
	}
}
