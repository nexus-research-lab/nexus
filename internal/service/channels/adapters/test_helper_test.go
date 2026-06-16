package adapters

import (
	"context"
	"io"
	"net/http"
	"strings"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type recordingIngressAcceptor struct {
	requests []channelcontract.IngressRequest
	err      error
}

func (r *recordingIngressAcceptor) Accept(
	_ context.Context,
	request channelcontract.IngressRequest,
) (*channelcontract.IngressResult, error) {
	r.requests = append(r.requests, request)
	if r.err != nil {
		return nil, r.err
	}
	return &channelcontract.IngressResult{
		Channel: request.Channel,
		AgentID: request.AgentID,
		ReqID:   request.ReqID,
	}, nil
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
