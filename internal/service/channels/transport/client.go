package transport

import (
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

var DefaultHTTPClient = &http.Client{
	Timeout: 45 * time.Second,
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   16,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
}

func NewWebsocketDialer() *websocket.Dialer {
	return &websocket.Dialer{
		Proxy:            websocketProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,
	}
}

func websocketProxyFromEnvironment(req *http.Request) (*url.URL, error) {
	if req == nil || req.URL == nil {
		return nil, nil
	}

	if req.URL.Scheme != "ws" && req.URL.Scheme != "wss" {
		return http.ProxyFromEnvironment(req)
	}

	proxyReq := new(http.Request)
	*proxyReq = *req
	proxyURL := *req.URL
	if req.URL.Scheme == "ws" {
		proxyURL.Scheme = "http"
	} else {
		proxyURL.Scheme = "https"
	}
	proxyReq.URL = &proxyURL
	return http.ProxyFromEnvironment(proxyReq)
}
