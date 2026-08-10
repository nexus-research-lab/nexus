package channels

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
)

type leaseAwareChannel struct {
	recordingDeliveryChannel

	mu      sync.Mutex
	ingress IngressAcceptor
}

func (c *leaseAwareChannel) SetIngress(ingress IngressAcceptor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ingress = ingress
}

func (c *leaseAwareChannel) ingressSnapshot() IngressAcceptor {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ingress
}

type recordingIngressAcceptor struct {
	mu       sync.Mutex
	requests []IngressRequest
}

func (a *recordingIngressAcceptor) Accept(
	_ context.Context,
	request IngressRequest,
) (*IngressResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.requests = append(a.requests, request)
	return &IngressResult{
		Channel: request.Channel,
		AgentID: request.AgentID,
	}, nil
}

func (a *recordingIngressAcceptor) requestSnapshot() []IngressRequest {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]IngressRequest(nil), a.requests...)
}

func TestRouterIngressLeaseRevokesStoppedGenerationBeforeCleanup(t *testing.T) {
	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, newChannelTestDB(t), nil, nil)
	delegate := &recordingIngressAcceptor{}
	router.SetIngress(delegate)
	if err := router.Start(t.Context()); err != nil {
		t.Fatalf("start router: %v", err)
	}
	defer router.Stop(t.Context())

	stopErr := errors.New("adapter stop failed")
	channel := &leaseAwareChannel{recordingDeliveryChannel: recordingDeliveryChannel{
		channelType: ChannelTypeTelegram,
		stopErr:     stopErr,
	}}
	if err := router.RegisterAndStartForOwner(t.Context(), "owner-a", channel); err != nil {
		t.Fatalf("register channel: %v", err)
	}
	staleIngress := channel.ingressSnapshot()
	if staleIngress == nil {
		t.Fatal("registered channel did not receive ingress lease")
	}
	if _, err := staleIngress.Accept(t.Context(), IngressRequest{
		Channel:     ChannelTypeTelegram,
		OwnerUserID: "forged-owner",
		Content:     "before delete",
	}); err != nil {
		t.Fatalf("active lease rejected ingress: %v", err)
	}

	if err := router.UnregisterForOwner(t.Context(), "owner-a", ChannelTypeTelegram); !errors.Is(err, stopErr) {
		t.Fatalf("unregister error = %v, want %v", err, stopErr)
	}
	if _, err := staleIngress.Accept(t.Context(), IngressRequest{
		Channel: ChannelTypeTelegram,
		Content: "after delete",
	}); !errors.Is(err, ErrIngressLeaseRevoked) {
		t.Fatalf("stale ingress error = %v, want revoked lease", err)
	}

	requests := delegate.requestSnapshot()
	if len(requests) != 1 {
		t.Fatalf("delegate accepted %d requests, want exactly the pre-delete request", len(requests))
	}
	if requests[0].OwnerUserID != "owner-a" || requests[0].Channel != ChannelTypeTelegram {
		t.Fatalf("lease did not bind server-side owner/channel: %+v", requests[0])
	}
}

func TestRouterIngressLeaseRejectsOldGenerationAfterHotReplacement(t *testing.T) {
	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, newChannelTestDB(t), nil, nil)
	delegate := &recordingIngressAcceptor{}
	router.SetIngress(delegate)
	if err := router.Start(t.Context()); err != nil {
		t.Fatalf("start router: %v", err)
	}
	defer router.Stop(t.Context())

	oldChannel := &leaseAwareChannel{recordingDeliveryChannel: recordingDeliveryChannel{
		channelType: ChannelTypeTelegram,
	}}
	if err := router.RegisterAndStartForOwner(t.Context(), "owner-a", oldChannel); err != nil {
		t.Fatalf("register old channel: %v", err)
	}
	oldIngress := oldChannel.ingressSnapshot()

	newChannel := &leaseAwareChannel{recordingDeliveryChannel: recordingDeliveryChannel{
		channelType: ChannelTypeTelegram,
	}}
	if err := router.RegisterAndStartForOwner(t.Context(), "owner-a", newChannel); err != nil {
		t.Fatalf("register replacement channel: %v", err)
	}
	newIngress := newChannel.ingressSnapshot()

	if _, err := oldIngress.Accept(t.Context(), IngressRequest{
		Channel: ChannelTypeTelegram,
		Content: "stale",
	}); !errors.Is(err, ErrIngressLeaseRevoked) {
		t.Fatalf("old generation error = %v, want revoked lease", err)
	}
	if _, err := newIngress.Accept(t.Context(), IngressRequest{
		Channel: ChannelTypeTelegram,
		Content: "current",
	}); err != nil {
		t.Fatalf("current generation rejected ingress: %v", err)
	}
	if _, err := newIngress.Accept(t.Context(), IngressRequest{
		Channel: ChannelTypeDiscord,
		Content: "cross-channel",
	}); !errors.Is(err, ErrIngressLeaseRevoked) {
		t.Fatalf("cross-channel error = %v, want revoked lease", err)
	}
	if requests := delegate.requestSnapshot(); len(requests) != 1 || requests[0].Content != "current" {
		t.Fatalf("delegate requests = %+v, want current generation only", requests)
	}
}
