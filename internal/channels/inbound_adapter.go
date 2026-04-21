package channels

import (
	"context"
	"strings"
)

// IngressAcceptor 表示通道入站消息的统一受理器。
type IngressAcceptor interface {
	Accept(context.Context, IngressRequest) (*IngressResult, error)
}

type ingressAwareChannel interface {
	SetIngress(IngressAcceptor)
}

func truncateChannelError(err error) string {
	if err == nil {
		return ""
	}
	text := strings.TrimSpace(err.Error())
	if text == "" {
		return "unknown error"
	}
	runes := []rune(text)
	if len(runes) <= 400 {
		return text
	}
	return string(runes[:400]) + "..."
}
