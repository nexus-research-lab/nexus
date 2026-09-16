package relay

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoomFilesStreamsBytesWithoutJSONWrapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil || string(body) != "hello" || r.ContentLength != 5 || r.URL.Path != "/api/relay/v1/rooms/room/files" || r.Header.Get("Authorization") != "Bearer token" || r.Header.Get("Cookie") != "" || r.Header.Get("Idempotency-Key") != "command" {
			t.Errorf("错误文件传输: %s %+v %v", body, r, err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"code":"0000","data":{"id":"file"}}`))
	}))
	defer server.Close()
	c, err := NewClient(server.URL, 0)
	if err != nil {
		t.Fatal(err)
	}
	response, err := c.RoomFiles(t.Context(), "token", "room", "", http.MethodPost, http.Header{"Idempotency-Key": []string{"command"}}, 5, strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatal(response.StatusCode)
	}
}
