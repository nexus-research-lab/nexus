// INPUT: 固定 Relay 地址、可信用户令牌和有界共享文件请求。
// OUTPUT: 流式文件响应；不缓存到 Nexus owner 的本机工作区。
// POS: 共享文件 transport，不能用于请求任意 URL。
package relay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"

	relaycontract "github.com/nexus-research-lab/nexus/internal/relay"
)

// DeliveryFile 下载当前投递引用的有界文件；验证摘要后才交给原生 Room 附件存储。
func (c *Client) DeliveryFile(ctx context.Context, token, deliveryID, leaseID string, file relaycontract.MessageAttachment) ([]byte, error) {
	for _, id := range []string{deliveryID, file.ID} {
		if _, err := requireResourceID(id, "file_delivery"); err != nil {
			return nil, err
		}
	}
	if file.Size < 0 || file.Size > 20<<20 {
		return nil, fmt.Errorf("在线附件超过本机上传限制")
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+relayAPIBase+"/node/deliveries/"+url.PathEscape(deliveryID)+"/files/"+url.PathEscape(file.ID), nil)
	if err != nil {
		return nil, err
	}
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("X-Delivery-Lease", leaseID)
	response, err := c.wsClient.Do(r)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("读取投递附件失败: HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, file.Size+1))
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(data)
	if int64(len(data)) != file.Size || hex.EncodeToString(hash[:]) != file.SHA256 {
		return nil, fmt.Errorf("在线附件长度或摘要不匹配")
	}
	return data, nil
}

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
