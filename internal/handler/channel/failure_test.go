package channel

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	channelspkg "github.com/nexus-research-lab/nexus/internal/service/channels"
)

func withRouteParam(ctx context.Context, key string, value string) context.Context {
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add(key, value)
	return context.WithValue(ctx, chi.RouteCtxKey, routeContext)
}

type fakeControl struct {
	currentLoginErr  error
	deletedAccountID string
	listPairingsErr  error
	upsertErr        error
}

func (f *fakeControl) ListChannels(context.Context, string) ([]channelspkg.ChannelConfigView, error) {
	return nil, nil
}

func (f *fakeControl) UpsertChannelConfig(context.Context, string, string, channelspkg.UpsertChannelConfigRequest) (*channelspkg.ChannelConfigView, error) {
	return nil, f.upsertErr
}

func (f *fakeControl) DeleteChannelConfig(context.Context, string, string) error {
	return nil
}

func (f *fakeControl) DeleteChannelAccount(_ context.Context, _ string, _ string, accountID string) (*channelspkg.ChannelConfigView, error) {
	f.deletedAccountID = accountID
	return nil, nil
}

func (f *fakeControl) StartChannelLogin(context.Context, string, string) (*channelspkg.ChannelLoginView, error) {
	return nil, nil
}

func (f *fakeControl) GetCurrentChannelLogin(context.Context, string, string) (*channelspkg.ChannelLoginView, error) {
	return nil, f.currentLoginErr
}

func (f *fakeControl) GetChannelLogin(context.Context, string, string, string) (*channelspkg.ChannelLoginView, error) {
	return nil, nil
}

func (f *fakeControl) SubmitChannelLoginVerifyCode(context.Context, string, string, string, channelspkg.SubmitChannelLoginVerifyCodeRequest) (*channelspkg.ChannelLoginView, error) {
	return nil, nil
}

func (f *fakeControl) ListPairings(context.Context, string, channelspkg.PairingQuery) ([]channelspkg.PairingView, error) {
	return nil, f.listPairingsErr
}

func (f *fakeControl) CreatePairing(context.Context, string, channelspkg.CreatePairingRequest) (*channelspkg.PairingView, error) {
	return nil, nil
}

func (f *fakeControl) UpdatePairing(context.Context, string, string, channelspkg.UpdatePairingRequest) (*channelspkg.PairingView, error) {
	return nil, nil
}

func (f *fakeControl) DeletePairing(context.Context, string, string) error {
	return nil
}

func (f *fakeControl) ResolveChannelOwnerByConfig(context.Context, string, string, string) (string, error) {
	return "", nil
}

func (f *fakeControl) PrepareFeishuIngress(context.Context, []byte, http.Header) (channelspkg.FeishuIngressPreparation, error) {
	return channelspkg.FeishuIngressPreparation{}, nil
}

func TestChannelMutationFailureUsesStableServiceFactsWithoutExposingCause(t *testing.T) {
	tests := []struct {
		name   string
		cause  error
		status int
		code   string
		effect protocol.FailureEffect
	}{
		{
			name:   "validation",
			cause:  errors.Join(channelspkg.ErrChannelControlInvalid, errors.New("secret-field is required")),
			status: http.StatusBadRequest,
			code:   "channel.save_config_invalid",
			effect: protocol.FailureEffectNotApplied,
		},
		{
			name:   "version conflict",
			cause:  channelspkg.ErrChannelControlVersionConflict,
			status: http.StatusConflict,
			code:   "channel.save_config_version_conflict",
			effect: protocol.FailureEffectNotApplied,
		},
		{
			name:   "unclassified mutation",
			cause:  errors.New("sql failed at /private/state.db token=secret"),
			status: http.StatusInternalServerError,
			code:   "channel.save_config_result_unknown",
			effect: protocol.FailureEffectUnknown,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, spec := channelMutationFailure(channelOperationSaveConfig, test.cause)
			if status != test.status || spec.Code != test.code || spec.Effect != test.effect {
				t.Fatalf("failure = status %d spec %+v", status, spec)
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
		"channel.save_config_result_unknown", protocol.FailureEffectUnknown)
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
		"channel.create_pairing_request_invalid", protocol.FailureEffectNotApplied)
}

func TestHandleDeleteChannelAccountDecodesEscapedAccountID(t *testing.T) {
	control := &fakeControl{}
	handler := New(handlershared.NewAPI(nil), nil, control)
	router := chi.NewRouter()
	router.Delete(
		"/nexus/v1/capability/channels/{channel_type}/accounts/{account_id}",
		handler.HandleDeleteChannelAccount,
	)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodDelete,
		"/nexus/v1/capability/channels/weixin-personal/accounts/bfd76ae5976a%40im.bot",
		nil,
	)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if control.deletedAccountID != "bfd76ae5976a@im.bot" {
		t.Fatalf("删除账号参数未按路径段解码: %q", control.deletedAccountID)
	}
}

func TestHandleListPairingsReadFailureNeverClaimsDataChanged(t *testing.T) {
	handler := New(handlershared.NewAPI(nil), nil, &fakeControl{
		listPairingsErr: errors.New("temporary query failure"),
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/nexus/v1/capability/channels/pairings", nil)
	handler.HandleListPairings(recorder, request)

	assertChannelFailure(t, recorder, http.StatusInternalServerError,
		"channel.list_pairings_failed", protocol.FailureEffectNotApplicable)
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
		"channel.read_login_state_ambiguous", protocol.FailureEffectNotApplicable)
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
		"channel.read_login_not_found", protocol.FailureEffectNotApplicable)
}

func assertChannelFailure(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	wantStatus int,
	wantCode string,
	wantEffect protocol.FailureEffect,
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
}
