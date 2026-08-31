package channel

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	channelspkg "github.com/nexus-research-lab/nexus/internal/service/channels"
)

func TestChannelMutationFailureUsesStableServiceFactsWithoutExposingCause(t *testing.T) {
	tests := []struct {
		name       string
		cause      error
		status     int
		code       string
		effect     protocol.FailureEffect
		resolution string
	}{
		{
			name:       "validation",
			cause:      errors.Join(channelspkg.ErrChannelControlInvalid, errors.New("secret-field is required")),
			status:     http.StatusBadRequest,
			code:       "channel.save_config_invalid",
			effect:     protocol.FailureEffectNotApplied,
			resolution: "channel.review_input",
		},
		{
			name:       "version conflict",
			cause:      channelspkg.ErrChannelControlVersionConflict,
			status:     http.StatusConflict,
			code:       "channel.save_config_version_conflict",
			effect:     protocol.FailureEffectNotApplied,
			resolution: "channel.reload_configs",
		},
		{
			name:       "unclassified mutation",
			cause:      errors.New("sql failed at /private/state.db token=secret"),
			status:     http.StatusInternalServerError,
			code:       "channel.save_config_result_unknown",
			effect:     protocol.FailureEffectUnknown,
			resolution: "channel.reload_configs",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, spec := channelMutationFailure(channelOperationSaveConfig, test.cause)
			if status != test.status || spec.Code != test.code || spec.Effect != test.effect {
				t.Fatalf("failure = status %d spec %+v", status, spec)
			}
			if spec.Resolution == nil || spec.Resolution.Action != test.resolution {
				t.Fatalf("resolution = %+v", spec.Resolution)
			}
			if strings.Contains(spec.Detail, "secret-field") || strings.Contains(spec.Detail, "/private/") {
				t.Fatalf("client detail leaked cause: %q", spec.Detail)
			}
		})
	}
}

func TestHandleUpsertChannelConfigUnknownFailureRequiresReconciliation(t *testing.T) {
	handler := New(handlershared.NewAPI(nil), nil, &fakeControl{
		upsertErr: errors.New("database path /private/nexus.db; credential=secret"),
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPut,
		"/nexus/v1/capability/channels/telegram",
		bytes.NewBufferString(`{"agent_id":"agent-a"}`),
	)
	request = request.WithContext(withRouteParam(request.Context(), "channel_type", "telegram"))
	handler.HandleUpsertChannelConfig(recorder, request)

	assertChannelFailure(t, recorder, http.StatusInternalServerError,
		"channel.save_config_result_unknown", protocol.FailureEffectUnknown, "channel.reload_configs")
	if strings.Contains(recorder.Body.String(), "/private/") || strings.Contains(recorder.Body.String(), "secret") {
		t.Fatalf("response leaked internal cause: %s", recorder.Body.String())
	}
}

func TestHandleCreatePairingMalformedJSONIsProvenNotApplied(t *testing.T) {
	handler := New(handlershared.NewAPI(nil), nil, &fakeControl{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/nexus/v1/capability/channels/pairings",
		bytes.NewBufferString(`{"agent_id":`),
	)
	handler.HandleCreatePairing(recorder, request)

	assertChannelFailure(t, recorder, http.StatusBadRequest,
		"channel.create_pairing_request_invalid", protocol.FailureEffectNotApplied, "channel.review_input")
}

func TestHandleListPairingsReadFailureNeverClaimsDataChanged(t *testing.T) {
	handler := New(handlershared.NewAPI(nil), nil, &fakeControl{
		listPairingsErr: errors.New("temporary query failure"),
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/nexus/v1/capability/channels/pairings", nil)
	handler.HandleListPairings(recorder, request)

	assertChannelFailure(t, recorder, http.StatusInternalServerError,
		"channel.list_pairings_failed", protocol.FailureEffectNotApplicable, "channel.reload_pairings")
}

func TestHandleGetCurrentChannelLoginFailsClosedWithoutExposingBinding(t *testing.T) {
	handler := New(handlershared.NewAPI(nil), nil, &fakeControl{
		currentLoginErr: errors.Join(
			channelspkg.ErrChannelLoginState,
			errors.New("authorization_binding=private-conversation-id"),
		),
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/nexus/v1/capability/channels/feishu/login",
		nil,
	)
	request = request.WithContext(withRouteParam(
		request.Context(),
		"channel_type",
		channelspkg.ChannelTypeFeishu,
	))
	handler.HandleGetCurrentChannelLogin(recorder, request)

	assertChannelFailure(t, recorder, http.StatusConflict,
		"channel.read_login_state_ambiguous", protocol.FailureEffectNotApplicable, "channel.reload_login")
	if strings.Contains(recorder.Body.String(), "private-conversation-id") ||
		strings.Contains(recorder.Body.String(), "authorization_binding") {
		t.Fatalf("response leaked authorization binding: %s", recorder.Body.String())
	}
}

func TestHandleGetCurrentChannelLoginAbsenceIsReadFactNotWriteOutcome(t *testing.T) {
	handler := New(handlershared.NewAPI(nil), nil, &fakeControl{
		currentLoginErr: channelspkg.ErrChannelLoginNotFound,
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/nexus/v1/capability/channels/feishu/login",
		nil,
	)
	request = request.WithContext(withRouteParam(
		request.Context(),
		"channel_type",
		channelspkg.ChannelTypeFeishu,
	))
	handler.HandleGetCurrentChannelLogin(recorder, request)

	assertChannelFailure(t, recorder, http.StatusNotFound,
		"channel.read_login_not_found", protocol.FailureEffectNotApplicable, "channel.reload_login")
}

func assertChannelFailure(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	wantStatus int,
	wantCode string,
	wantEffect protocol.FailureEffect,
	wantAction string,
) {
	t.Helper()
	if recorder.Code != wantStatus {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data struct {
			Failure protocol.FailureCore `json:"failure"`
		} `json:"data"`
	}
	if err := handlershared.DecodeJSONBody(bytes.NewReader(recorder.Body.Bytes()), &response, false); err != nil {
		t.Fatalf("decode response: %v body=%s", err, recorder.Body.String())
	}
	if response.Data.Failure.Code != wantCode || response.Data.Failure.Effect != wantEffect {
		t.Fatalf("failure = %+v", response.Data.Failure)
	}
	if response.Data.Failure.Resolution == nil || response.Data.Failure.Resolution.Action != wantAction {
		t.Fatalf("resolution = %+v", response.Data.Failure.Resolution)
	}
}
