package runtime

import (
	"context"
	"errors"
	"maps"
	"slices"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

var (
	// ErrNoRunningRound 表示当前 session 没有可接收排队输入的运行中 round。
	ErrNoRunningRound = errors.New("runtime session has no running round")
	// ErrStreamingInputUnsupported 表示底层 client 不支持流式排队输入。
	ErrStreamingInputUnsupported = errors.New("runtime client does not support streaming input")
)

type streamingInputClient interface {
	SendContent(context.Context, any, *string, string) error
}

type streamingInputOptionsClient interface {
	SendContentWithOptions(context.Context, any, *string, string, sdkprotocol.OutboundMessageOptions) error
}

// SendContentToRunningRound 把新输入排入当前运行中的 SDK 流。
func (m *Manager) SendContentToRunningRound(ctx context.Context, sessionKey string, content any) ([]string, error) {
	m.mu.Lock()
	state, ok := m.sessions[sessionKey]
	if !ok || state == nil || state.Client == nil || len(state.RunningRounds) == 0 {
		m.mu.Unlock()
		return nil, ErrNoRunningRound
	}
	roundIDs := slices.Sorted(maps.Keys(state.RunningRounds))
	client := state.Client
	m.touchStateLocked(state)
	m.mu.Unlock()

	if err := SendClientContent(ctx, client, content); err != nil {
		return roundIDs, err
	}
	return roundIDs, nil
}

// SendClientContent 通过 SDK streaming input 向活动 client 投递用户输入。
func SendClientContent(ctx context.Context, client Client, content any) error {
	return SendClientContentWithOptions(ctx, client, content, sdkprotocol.OutboundMessageOptions{})
}

// SendClientContentWithOptions 通过 SDK streaming input 投递带附加语义的用户输入。
func SendClientContentWithOptions(ctx context.Context, client Client, content any, options sdkprotocol.OutboundMessageOptions) error {
	if client == nil {
		return ErrNoRunningRound
	}
	if sender, ok := client.(streamingInputOptionsClient); ok {
		return sender.SendContentWithOptions(ctx, content, nil, "", options)
	}
	sender, ok := client.(streamingInputClient)
	if !ok {
		return ErrStreamingInputUnsupported
	}
	return sender.SendContent(ctx, content, nil, "")
}

type queryContentClient interface {
	QueryContent(context.Context, any) error
}

type queryContentOptionsClient interface {
	QueryContentWithOptions(context.Context, any, sdkprotocol.OutboundMessageOptions) error
}

// QueryClientContentWithOptions 通过 SDK client 启动一轮带附加语义的用户输入。
func QueryClientContentWithOptions(ctx context.Context, client Client, content any, options sdkprotocol.OutboundMessageOptions) error {
	if client == nil {
		return ErrNoRunningRound
	}
	if prompt, ok := content.(string); ok {
		if sender, ok := client.(interface {
			QueryWithOptions(context.Context, string, sdkprotocol.OutboundMessageOptions) error
		}); ok {
			return sender.QueryWithOptions(ctx, prompt, options)
		}
		return client.Query(ctx, prompt)
	}
	if sender, ok := client.(queryContentOptionsClient); ok {
		return sender.QueryContentWithOptions(ctx, content, options)
	}
	sender, ok := client.(queryContentClient)
	if !ok {
		return ErrStreamingInputUnsupported
	}
	return sender.QueryContent(ctx, content)
}
