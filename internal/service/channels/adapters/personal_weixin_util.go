package adapters

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"strconv"
	"strings"
	"time"
)

func personalWeixinTextContent(message personalWeixinMessage) string {
	parts := make([]string, 0, len(message.ItemList))
	for _, item := range message.ItemList {
		if item.Type != personalWeixinItemTypeText {
			continue
		}
		if text := strings.TrimSpace(item.TextItem.Text); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func normalizePersonalWeixinBaseURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return DefaultPersonalWeixinBaseURL
	}
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	return strings.TrimRight(value, "/")
}

func randomPersonalWeixinUIN() string {
	buffer := make([]byte, 4)
	if _, err := rand.Read(buffer); err != nil {
		return base64.StdEncoding.EncodeToString([]byte(strconv.FormatInt(time.Now().UnixNano(), 10)))
	}
	value := binary.BigEndian.Uint32(buffer)
	return base64.StdEncoding.EncodeToString([]byte(strconv.FormatUint(uint64(value), 10)))
}

func waitPersonalWeixinRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
