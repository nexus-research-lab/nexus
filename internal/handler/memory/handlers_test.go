package memory_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
)

func TestMemoryHandlersLifecycle(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	cfg.MemoryEnabled = true
	cfg.MemoryAutoRecall = true
	cfg.MemoryAutoExtract = true
	cfg.MemoryMaxResults = 5
	cfg.MemoryScoreThreshold = 0.08
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}

	addRecorder := serveJSON(t, server, http.MethodPost, "/nexus/v1/agents/nexus/memory/items", []byte(`{
		"title": "中文注释偏好",
		"content": "以后默认复杂逻辑注释使用中文。",
		"status": "candidate"
	}`))
	if addRecorder.Code != http.StatusOK {
		t.Fatalf("新增记忆状态码不正确: got=%d body=%s", addRecorder.Code, addRecorder.Body.String())
	}
	var added struct {
		Data struct {
			EntryID string `json:"entry_id"`
			Status  string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(addRecorder.Body.Bytes(), &added); err != nil {
		t.Fatalf("解析新增响应失败: %v", err)
	}
	if added.Data.EntryID == "" || added.Data.Status != "candidate" {
		t.Fatalf("新增记忆响应不完整: %+v", added.Data)
	}

	searchRecorder := serveJSON(t, server, http.MethodGet, "/nexus/v1/agents/nexus/memory/search?q=中文%20注释", nil)
	if searchRecorder.Code != http.StatusOK {
		t.Fatalf("搜索记忆状态码不正确: got=%d body=%s", searchRecorder.Code, searchRecorder.Body.String())
	}
	var searched struct {
		Data struct {
			Items []struct {
				EntryID string `json:"entry_id"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(searchRecorder.Body.Bytes(), &searched); err != nil {
		t.Fatalf("解析搜索响应失败: %v", err)
	}
	if len(searched.Data.Items) == 0 || searched.Data.Items[0].EntryID != added.Data.EntryID {
		t.Fatalf("搜索结果未命中新增记忆: %+v", searched.Data.Items)
	}

	ignoreRecorder := serveJSON(
		t,
		server,
		http.MethodPost,
		"/nexus/v1/agents/nexus/memory/items/"+added.Data.EntryID+"/ignore",
		[]byte(`{"note":"测试忽略"}`),
	)
	if ignoreRecorder.Code != http.StatusOK {
		t.Fatalf("忽略记忆状态码不正确: got=%d body=%s", ignoreRecorder.Code, ignoreRecorder.Body.String())
	}

	searchAfterIgnore := serveJSON(t, server, http.MethodGet, "/nexus/v1/agents/nexus/memory/search?q=中文%20注释", nil)
	if searchAfterIgnore.Code != http.StatusOK {
		t.Fatalf("忽略后搜索状态码不正确: got=%d body=%s", searchAfterIgnore.Code, searchAfterIgnore.Body.String())
	}
	var ignoredSearch struct {
		Data struct {
			Items []struct {
				EntryID string `json:"entry_id"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(searchAfterIgnore.Body.Bytes(), &ignoredSearch); err != nil {
		t.Fatalf("解析忽略后搜索响应失败: %v", err)
	}
	for _, item := range ignoredSearch.Data.Items {
		if item.EntryID == added.Data.EntryID {
			t.Fatalf("忽略后的记忆不应继续被召回: %+v", ignoredSearch.Data.Items)
		}
	}

	cleanupRecorder := serveJSON(t, server, http.MethodPost, "/nexus/v1/agents/nexus/memory/cleanup", []byte(`{}`))
	if cleanupRecorder.Code != http.StatusOK {
		t.Fatalf("清理记忆状态码不正确: got=%d body=%s", cleanupRecorder.Code, cleanupRecorder.Body.String())
	}
	var cleanup struct {
		Data struct {
			RemovedSessionFiles int `json:"removed_session_files"`
			RemovedCheckpoints  int `json:"removed_checkpoints"`
		} `json:"data"`
	}
	if err := json.Unmarshal(cleanupRecorder.Body.Bytes(), &cleanup); err != nil {
		t.Fatalf("解析清理响应失败: %v", err)
	}
	if cleanup.Data.RemovedSessionFiles != 0 || cleanup.Data.RemovedCheckpoints != 0 {
		t.Fatalf("无孤立数据时不应误清理: %+v", cleanup.Data)
	}
}

func serveJSON(t *testing.T, server *serverapp.Server, method string, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader(body)
	}
	request := httptest.NewRequest(method, path, reader)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, request)
	return recorder
}
