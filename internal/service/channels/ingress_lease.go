// INPUT: Router 发布的 owner+channel+generation 与通道实例提交的 ingress。
// OUTPUT: 仅当前已启动 generation 可转发的、身份被服务端固定的 ingress。
// POS: Channel 热替换/删除的最终撤权栅栏；Stop 只负责清理，不承担授权。
package channels

import (
	"context"
	"errors"
	"fmt"
)

var ErrIngressLeaseRevoked = errors.New("channel ingress lease has been revoked")

type channelIngressLease struct {
	router      *Router
	delegate    IngressAcceptor
	ownerUserID string
	channelType string
	generation  uint64
}

func (l *channelIngressLease) Accept(
	ctx context.Context,
	request IngressRequest,
) (*IngressResult, error) {
	if l == nil || l.router == nil || l.delegate == nil ||
		!l.router.isIngressGenerationActive(l.ownerUserID, l.channelType, l.generation) {
		return nil, ErrIngressLeaseRevoked
	}
	requestChannel := normalizeChannelType(request.Channel)
	if requestChannel != "" && requestChannel != l.channelType {
		return nil, fmt.Errorf(
			"%w: leased channel=%s request channel=%s",
			ErrIngressLeaseRevoked,
			l.channelType,
			requestChannel,
		)
	}
	// Adapter 提供的 owner/channel 不是权限真相源。每一代 lease 都把它们
	// 固定为 Router 发布时的服务端值，覆盖残留实例或错误 adapter 的声明。
	request.OwnerUserID = l.ownerUserID
	request.Channel = l.channelType
	return l.delegate.Accept(ctx, request)
}

func (r *Router) ingressForRegisteredChannel(
	entry *registeredChannel,
	delegate IngressAcceptor,
) IngressAcceptor {
	if r == nil || entry == nil {
		return &channelIngressLease{}
	}
	return &channelIngressLease{
		router:      r,
		delegate:    delegate,
		ownerUserID: entry.ownerUserID,
		channelType: entry.channelType,
		generation:  entry.generation,
	}
}

func (r *Router) isIngressGenerationActive(
	ownerUserID string,
	channelType string,
	generation uint64,
) bool {
	if r == nil || generation == 0 {
		return false
	}
	key := channelRouteKey(ownerUserID, channelType)
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry := r.channels[key]
	return r.running &&
		entry != nil &&
		entry.started &&
		entry.generation == generation
}
