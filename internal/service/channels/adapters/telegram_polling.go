package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	channeltransport "github.com/nexus-research-lab/nexus/internal/service/channels/transport"
)

const telegramIngressMaxAttempts = 3

type telegramPollCursor struct {
	offset         int
	failedUpdateID int
	attempts       int
}

func (c *TelegramChannel) pollUpdates(ctx context.Context) {
	defer c.wg.Done()

	cursor := telegramPollCursor{}
	lastErrText := ""
	lastErrLoggedAt := time.Time{}
	for {
		if ctx.Err() != nil {
			return
		}
		updates, _, err := c.fetchUpdates(ctx, cursor.offset)
		if err == nil {
			err = c.handlePolledUpdates(ctx, updates, &cursor)
		}
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			errText := strings.TrimSpace(err.Error())
			now := time.Now()
			if errText != lastErrText || now.Sub(lastErrLoggedAt) >= 30*time.Second {
				c.loggerFor(ctx).Warn("Telegram 接收消息失败",
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
			c.loggerFor(ctx).Info("Telegram 接收消息已恢复",
				"owner_user_id", c.ownerUserID,
			)
			lastErrText = ""
			lastErrLoggedAt = time.Time{}
		}
	}
}

// handlePolledUpdates 逐条推进游标；安全重试有上限，未知结果只提示、不重跑。
func (c *TelegramChannel) handlePolledUpdates(ctx context.Context, updates []telegramUpdate, cursor *telegramPollCursor) error {
	for _, update := range updates {
		err := c.handleUpdate(ctx, update)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil && cursor.retry(update.UpdateID, err) {
			return err
		}
		if err != nil {
			c.reportIngressFailure(ctx, update, err)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		cursor.offset = max(cursor.offset, update.UpdateID+1)
		cursor.attempts = 0
	}
	return nil
}

func (cursor *telegramPollCursor) retry(updateID int, err error) bool {
	if cursor.failedUpdateID != updateID {
		cursor.attempts = 0
	}
	cursor.failedUpdateID = updateID
	cursor.attempts++
	var retryable *channelcontract.RetryableIngressError
	return errors.As(err, &retryable) && cursor.attempts < telegramIngressMaxAttempts
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
