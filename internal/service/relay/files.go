// INPUT: 固定 Relay 地址、可信用户令牌和有界共享文件请求。
// OUTPUT: 流式文件响应；不缓存到 Nexus owner 的本机工作区。
// POS: 共享文件 transport，不能用于请求任意 URL。
package relay

import (
	"context"
	"io"
	"net/http"
	"net/url"
)

func (c *Client) RoomFiles(ctx context.Context, token, roomID, fileID, method string, headers http.Header, size int64, body io.Reader) (*http.Response, error) {
	if _, err := requireResourceID(roomID, "room_id"); err != nil {
		return nil, err
	}
	path := "/rooms/" + url.PathEscape(roomID) + "/files"
	if fileID != "" {
		if _, err := requireResourceID(fileID, "file_id"); err != nil {
			return nil, err
		}
		path += "/" + url.PathEscape(fileID)
	}
	r, err := http.NewRequestWithContext(ctx, method, c.baseURL+relayAPIBase+path, body)
	if err != nil {
		return nil, err
	}
	r.ContentLength = size
	r.Header.Set("Authorization", "Bearer "+token)
	for _, name := range []string{"Content-Type", "X-File-Name", "X-File-SHA256", "Idempotency-Key"} {
		r.Header.Set(name, headers.Get(name))
	}
	return c.wsClient.Do(r)
}
