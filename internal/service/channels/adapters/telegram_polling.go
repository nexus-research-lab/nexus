package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	channeltransport "github.com/nexus-research-lab/nexus/internal/service/channels/transport"
)

func (c *TelegramChannel) pollUpdates(ctx context.Context) {
	defer c.wg.Done()

	offset := 0
	lastErrText := ""
	lastErrLoggedAt := time.Time{}
	for {
		if ctx.Err() != nil {
			return
		}
		updates, nextOffset, err := c.fetchUpdates(ctx, offset)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			errText := strings.TrimSpace(err.Error())
			now := time.Now()
			if errText != lastErrText || now.Sub(lastErrLoggedAt) >= 30*time.Second {
				c.loggerFor(ctx).Warn("Telegram getUpdates 失败",
					"owner_user_id", c.ownerUserID,
					"err", c.redactError(err),
				)
				lastErrText = errText
				lastErrLoggedAt = now
			}
			timer := time.NewTimer(2 * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			continue
		}
		if lastErrText != "" {
			c.loggerFor(ctx).Info("Telegram getUpdates 已恢复",
				"owner_user_id", c.ownerUserID,
			)
			lastErrText = ""
			lastErrLoggedAt = time.Time{}
		}
		offset = nextOffset
		for _, update := range updates {
			c.handleUpdate(ctx, update)
		}
	}
}

func (c *TelegramChannel) fetchUpdates(ctx context.Context, offset int) ([]telegramUpdate, int, error) {
	payload := map[string]any{
		"offset":          offset,
		"timeout":         30,
		"allowed_updates": []string{"message", "edited_message"},
	}
	response, err := channeltransport.DoJSON(
		ctx,
		c.client,
		http.MethodPost,
		strings.TrimRight(c.baseURL, "/")+"/bot"+c.token+"/getUpdates",
		payload,
		nil,
	)
	if err != nil {
		return nil, offset, c.redactError(err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, offset, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, offset, fmt.Errorf(
			"telegram getUpdates failed: status=%d body=%s",
			response.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}

	var envelope telegramUpdatesEnvelope
	if err = json.Unmarshal(body, &envelope); err != nil {
		return nil, offset, err
	}
	if !envelope.OK {
		return nil, offset, fmt.Errorf("telegram getUpdates returned not ok: %s", strings.TrimSpace(envelope.Description))
	}

	nextOffset := offset
	for _, update := range envelope.Result {
		if update.UpdateID >= nextOffset {
			nextOffset = update.UpdateID + 1
		}
	}
	return envelope.Result, nextOffset, nil
}
