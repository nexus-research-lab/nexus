// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：round_executor.go
// @Date   ：2026/04/21 20:05:00
// @Author ：leemysw
// 2026/04/21 20:05:00   Create
// =====================================================

package runtime

import (
	"context"
	"errors"
	"strings"

	sessionmodel "github.com/nexus-research-lab/nexus/internal/model/session"
	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

var (
	// ErrRoundInterrupted 表示 round 在收到终态前被中断。
	ErrRoundInterrupted = errors.New("round interrupted")
	// ErrRoundStreamClosedBeforeTerminal 表示 SDK 在产出终态前提前结束消息流。
	ErrRoundStreamClosedBeforeTerminal = errors.New("round stream closed before terminal")
)

// RoundMapResult 表示单条 SDK 消息映射后的统一结果。
type RoundMapResult struct {
	Events          []protocol.EventMessage
	DurableMessages []sessionmodel.Message
	TerminalStatus  string
	ResultSubtype   string
}

// RoundMapper 负责把 SDK 消息映射成统一事件与 durable 消息。
type RoundMapper interface {
	Map(sdkprotocol.ReceivedMessage, ...string) (RoundMapResult, error)
	SessionID() string
}

// RoundExecutionRequest 表示执行单轮查询所需的回调与依赖。
type RoundExecutionRequest struct {
	Query                  string
	Client                 Client
	Mapper                 RoundMapper
	InterruptReason        func() string
	SyncSessionID          func(string) error
	HandleDurableMessage   func(sessionmodel.Message) error
	EmitEvent              func(protocol.EventMessage) error
	ObserveIncomingMessage func(sdkprotocol.ReceivedMessage)
}

// RoundExecutionResult 表示 round 执行的终态结果。
type RoundExecutionResult struct {
	TerminalStatus string
	ResultSubtype  string
}

// ExecuteRound 统一执行 query -> receive -> map -> persist -> emit 的主链路。
func ExecuteRound(
	ctx context.Context,
	request RoundExecutionRequest,
) (RoundExecutionResult, error) {
	if request.Client == nil {
		return RoundExecutionResult{}, errors.New("round client is required")
	}
	if request.Mapper == nil {
		return RoundExecutionResult{}, errors.New("round mapper is required")
	}

	if err := request.Client.Query(ctx, request.Query); err != nil {
		if ctx.Err() != nil || errors.Is(err, context.Canceled) {
			return RoundExecutionResult{}, ErrRoundInterrupted
		}
		return RoundExecutionResult{}, err
	}

	messageCh := request.Client.ReceiveMessages(ctx)
	for {
		select {
		case <-ctx.Done():
			return RoundExecutionResult{}, ErrRoundInterrupted
		case incoming, ok := <-messageCh:
			if !ok {
				if shouldTreatAsInterrupted(ctx, request.InterruptReason) {
					return RoundExecutionResult{}, ErrRoundInterrupted
				}
				return RoundExecutionResult{}, ErrRoundStreamClosedBeforeTerminal
			}
			if request.ObserveIncomingMessage != nil {
				request.ObserveIncomingMessage(incoming)
			}

			mapResult, err := request.Mapper.Map(incoming, resolveInterruptReason(request.InterruptReason))
			if err != nil {
				return RoundExecutionResult{}, err
			}

			sessionID := resolveSessionID(
				request.Mapper.SessionID(),
				incoming.SessionID,
				request.Client.SessionID(),
			)
			if request.SyncSessionID != nil && sessionID != "" {
				if err := request.SyncSessionID(sessionID); err != nil {
					return RoundExecutionResult{}, err
				}
			}

			for _, messageValue := range mapResult.DurableMessages {
				if messageValue == nil {
					continue
				}
				if sessionID != "" && strings.TrimSpace(messageString(messageValue["session_id"])) == "" {
					messageValue["session_id"] = sessionID
				}
				if request.HandleDurableMessage != nil {
					if err := request.HandleDurableMessage(messageValue); err != nil {
						return RoundExecutionResult{}, err
					}
				}
			}

			for _, event := range mapResult.Events {
				if request.EmitEvent != nil {
					if err := request.EmitEvent(event); err != nil {
						return RoundExecutionResult{}, err
					}
				}
			}

			if strings.TrimSpace(mapResult.TerminalStatus) != "" {
				return RoundExecutionResult{
					TerminalStatus: strings.TrimSpace(mapResult.TerminalStatus),
					ResultSubtype:  strings.TrimSpace(mapResult.ResultSubtype),
				}, nil
			}
		}
	}
}

func shouldTreatAsInterrupted(ctx context.Context, interruptReason func() string) bool {
	return ctx.Err() != nil || strings.TrimSpace(resolveInterruptReason(interruptReason)) != ""
}

func resolveInterruptReason(interruptReason func() string) string {
	if interruptReason == nil {
		return ""
	}
	return strings.TrimSpace(interruptReason())
}

func resolveSessionID(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func messageString(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return typed
}
