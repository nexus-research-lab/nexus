package adapters

import (
	"net/http"
	"strings"
	"sync"
	"time"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	channeltransport "github.com/nexus-research-lab/nexus/internal/service/channels/transport"
	dingclient "github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
	"golang.org/x/sync/singleflight"
)

type DingTalkChannel struct {
	clientID     string
	clientSecret string
	robotCode    string
	client       *http.Client
	baseURL      string
	streamHost   string
	ownerUserID  string

	mu             sync.RWMutex
	ingress        channelcontract.IngressAcceptor
	accessToken    string
	tokenExpiresAt time.Time
	stream         *dingclient.StreamClient
	tokenFlight    singleflight.Group
}

func NewDingTalkChannel(clientID string, clientSecret string, robotCode string, client *http.Client) *DingTalkChannel {
	if client == nil {
		client = channeltransport.DefaultHTTPClient
	}
	return &DingTalkChannel{
		clientID:     strings.TrimSpace(clientID),
		clientSecret: strings.TrimSpace(clientSecret),
		robotCode:    strings.TrimSpace(robotCode),
		client:       client,
		baseURL:      "https://api.dingtalk.com",
		streamHost:   "https://api.dingtalk.com",
	}
}

func (c *DingTalkChannel) WithOwner(ownerUserID string) *DingTalkChannel {
	c.ownerUserID = strings.TrimSpace(ownerUserID)
	return c
}

func (c *DingTalkChannel) WithBaseURL(baseURL string) *DingTalkChannel {
	if baseURL = strings.TrimSpace(baseURL); baseURL != "" {
		c.baseURL = normalizeDingTalkBaseURL(baseURL)
	}
	return c
}

func (c *DingTalkChannel) WithStreamHost(streamHost string) *DingTalkChannel {
	if streamHost = strings.TrimSpace(streamHost); streamHost != "" {
		c.streamHost = normalizeDingTalkBaseURL(streamHost)
	}
	return c
}

func (c *DingTalkChannel) BaseURL() string {
	return c.baseURL
}

func (c *DingTalkChannel) StreamHost() string {
	return c.streamHost
}

func (c *DingTalkChannel) ChannelType() string {
	return channelcontract.ChannelTypeDingTalk
}

func (c *DingTalkChannel) SetIngress(ingress channelcontract.IngressAcceptor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ingress = ingress
}

func (c *DingTalkChannel) currentIngress() channelcontract.IngressAcceptor {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ingress
}
