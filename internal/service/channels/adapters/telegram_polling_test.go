package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
)

type telegramPollingIngress func(context.Context, channelcontract.IngressRequest) (*channelcontract.IngressResult, error)

func (fn telegramPollingIngress) Accept(ctx context.Context, request channelcontract.IngressRequest) (*channelcontract.IngressResult, error) {
	return fn(ctx, request)
}

func TestTelegramPollingFailureDoesNotBlockOtherChats(t *testing.T) {
	for _, test := range []struct {
		name                             string
		retryable, recovers, noticeFails bool
		attempts                         int
		offsets                          string
	}{
		{name: "准备阶段短暂故障", retryable: true, recovers: true, attempts: 2, offsets: "[0 0 102]"},
		{name: "准备阶段持续故障", retryable: true, attempts: 3, offsets: "[0 0 0 102]"},
		{name: "结果未知不重跑", attempts: 1, offsets: "[0 102]"},
		{name: "通知失败不回退", noticeFails: true, attempts: 1, offsets: "[0 102]"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 12*time.Second)
			defer cancel()
			attempts, healthy, notices := 0, 0, 0
			offsets := []int{}
			rounds := []string{}
			transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if strings.HasSuffix(r.URL.Path, "/sendMessage") {
					notices++
					if body["chat_id"] != "-10" || body["message_thread_id"] != float64(77) {
						t.Errorf("错误通知发往其他会话: %+v", body)
					}
					text, _ := body["text"].(string)
					if strings.Contains(text, "provider_secret") || !strings.Contains(text, "42") {
						t.Errorf("通知泄露错误或缺少消息标识: %s", text)
					}
					if test.retryable && !strings.Contains(text, "本次接收失败") {
						t.Errorf("准备失败被当成未知执行: %s", text)
					}
					if !test.retryable && !strings.Contains(text, "未能确认") {
						t.Errorf("未知结果被当成确定失败: %s", text)
					}
					if test.noticeFails {
						response := jsonResponse(`{"ok":false}`)
						response.StatusCode = http.StatusInternalServerError
						return response, nil
					}
					return jsonResponse(`{"ok":true,"result":{"message_id":90}}`), nil
				}
				offset := int(body["offset"].(float64))
				offsets = append(offsets, offset)
				if offset == 102 {
					cancel()
					return nil, context.Canceled
				}
				return jsonResponse(`{"ok":true,"result":[{"update_id":100,"message":{"message_id":42,"text":"bad","message_thread_id":77,"from":{"id":7},"chat":{"id":-10,"type":"supergroup"}}},{"update_id":101,"message":{"message_id":43,"text":"good","from":{"id":8},"chat":{"id":9,"type":"private"}}}]}`), nil
			})
			channel := NewTelegramChannel("test", &http.Client{Transport: transport})
			channel.SetIngress(telegramPollingIngress(func(_ context.Context, request channelcontract.IngressRequest) (*channelcontract.IngressResult, error) {
				if request.Content == "good" {
					healthy++
					return &channelcontract.IngressResult{}, nil
				}
				attempts++
				rounds = append(rounds, request.RoundID)
				if request.ReqID != "update:100" {
					t.Errorf("事件身份变化: %s", request.ReqID)
				}
				if test.recovers && attempts == 2 {
					return &channelcontract.IngressResult{}, nil
				}
				err := errors.New("provider_secret")
				if test.retryable {
					err = &channelcontract.RetryableIngressError{Err: err}
				}
				return nil, err
			}))
			channel.wg.Add(1)
			channel.pollUpdates(ctx)
			if attempts != test.attempts || healthy != 1 || fmt.Sprint(offsets) != test.offsets {
				t.Fatalf("attempts=%d healthy=%d offsets=%v", attempts, healthy, offsets)
			}
			expectedNotices := 1
			if test.recovers {
				expectedNotices = 0
			}
			if notices != expectedNotices {
				t.Fatalf("通知次数=%d", notices)
			}
			for _, round := range rounds {
				if round != "telegram_100" {
					t.Fatalf("轮次变化: %v", rounds)
				}
			}
		})
	}
}

func TestTelegramPollingCancellationDoesNotAcknowledgeUpdate(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	polls := 0
	channel := NewTelegramChannel("test", &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		polls++
		if polls != 1 || !strings.HasSuffix(r.URL.Path, "/getUpdates") {
			t.Errorf("取消后继续确认或发送通知: %s", r.URL.Path)
		}
		return jsonResponse(`{"ok":true,"result":[{"update_id":100,"message":{"message_id":42,"text":"hello","from":{"id":7},"chat":{"id":8,"type":"private"}}}]}`), nil
	})})
	channel.SetIngress(telegramPollingIngress(func(context.Context, channelcontract.IngressRequest) (*channelcontract.IngressResult, error) {
		cancel()
		return nil, context.Canceled
	}))
	channel.wg.Add(1)
	channel.pollUpdates(ctx)
}
