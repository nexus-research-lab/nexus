// INPUT: 浏览器 Session Cookie、Control internal API 与 Ed25519 公钥。
// OUTPUT: 经签名验证并绑定本机 owner key 的 Nexus Principal。
// POS: Nexus Server 到独立 nexus-control 的唯一认证 adapter。
package auth

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/runtimeadmission"
)

const controlAPIBase = "/api/control/v1"

// ControlAuthority 通过 HTTP 消费 Control，不读取其数据库。
type ControlAuthority struct {
	config           config.Config
	baseURL          string
	client           *http.Client
	bindings         *controlBindingStore
	verifier         controlPrincipalVerifier
	runtimeAdmission RuntimeAdmissionCoordinator
	humanSessionMu   sync.RWMutex
	leaseMu          sync.RWMutex
	leases           map[string]controlCachedLease
}

type controlCachedLease struct {
	principal *Principal
	state     State
	expiresAt time.Time
}

func NewControlAuthority(
	cfg config.Config,
	db *sql.DB,
	runtimeAdmission RuntimeAdmissionCoordinator,
) *ControlAuthority {
	timeout := time.Duration(cfg.ControlRequestTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &ControlAuthority{
		config:  cfg,
		baseURL: strings.TrimRight(strings.TrimSpace(cfg.ControlURL), "/"),
		client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		bindings:         newControlBindingStore(cfg.DatabaseDriver, db),
		runtimeAdmission: runtimeAdmission,
		leases:           make(map[string]controlCachedLease),
		verifier: controlPrincipalVerifier{
			encodedKey: cfg.ControlPrincipalPublicKey,
			keyFile:    cfg.ControlPrincipalPublicKeyFile,
			audience:   strings.TrimSpace(cfg.ControlPrincipalAudience),
			now:        func() time.Time { return time.Now().UTC() },
		},
	}
}

// ValidateControlConfig 在 Server 接收请求前固定服务凭据和验签材料。
func ValidateControlConfig(cfg config.Config) error {
	if strings.EqualFold(strings.TrimSpace(cfg.AppMode), "desktop") {
		return nil
	}
	endpoint, err := url.Parse(strings.TrimSpace(cfg.ControlURL))
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
		return errors.New("NEXUS_CONTROL_URL 必须是完整的 http(s) URL")
	}
	if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return errors.New("NEXUS_CONTROL_URL 只支持 http 或 https")
	}
	if endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return errors.New("NEXUS_CONTROL_URL 不能包含凭据、查询参数或 fragment")
	}
	if len(strings.TrimSpace(cfg.ControlServiceToken)) < 32 {
		return errors.New("NEXUS_CONTROL_SERVICE_TOKEN 至少需要 32 个字符")
	}
	verifier := controlPrincipalVerifier{
		encodedKey: cfg.ControlPrincipalPublicKey,
		keyFile:    cfg.ControlPrincipalPublicKeyFile,
		audience:   strings.TrimSpace(cfg.ControlPrincipalAudience),
		now:        func() time.Time { return time.Now().UTC() },
	}
	if verifier.audience == "" {
		return errors.New("NEXUS_CONTROL_PRINCIPAL_AUDIENCE 不能为空")
	}
	verifier.once.Do(verifier.load)
	return verifier.err
}

func (a *ControlAuthority) InspectRequest(
	ctx context.Context,
	request *http.Request,
) (*Principal, State, error) {
	return a.exchange(ctx, a.extractSessionToken(request))
}

func (a *ControlAuthority) BuildStatusPayload(
	ctx context.Context,
	request *http.Request,
) (StatusPayload, error) {
	principal, state, err := a.InspectRequest(ctx, request)
	if err != nil {
		return StatusPayload{}, err
	}
	result := StatusPayload{
		AuthRequired:         true,
		PasswordLoginEnabled: state.PasswordLoginEnabled,
		Authenticated:        principal != nil,
		SetupRequired:        state.SetupRequired,
		SetupEnabled:         state.SetupEnabled,
	}
	if principal == nil {
		return result, nil
	}
	result.Username = stringPointer(principal.Username)
	result.UserID = stringPointer(principal.ControlUserID)
	result.DisplayName = stringPointer(principal.DisplayName)
	result.Role = stringPointer(principal.Role)
	result.Avatar = stringPointer(principal.Avatar)
	result.AuthMethod = stringPointer(principal.AuthMethod)
	return result, nil
}

func (a *ControlAuthority) extractSessionToken(request *http.Request) string {
	if request == nil {
		return ""
	}
	cookieName := strings.TrimSpace(a.config.AuthSessionCookieName)
	if cookieName == "" {
		cookieName = "nexus_session"
	}
	cookie, err := request.Cookie(cookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func (a *ControlAuthority) BeginAgentRuntimeAdmission(
	ctx context.Context,
) (*runtimeadmission.Lease, error) {
	lease, err := beginAgentRuntimeAdmission(ctx, a.runtimeAdmission)
	if err != nil {
		return nil, err
	}
	state, err := a.state(lease.Context())
	if err != nil {
		lease.Release()
		return nil, err
	}
	if state.SetupRequired {
		lease.Release()
		return nil, errors.New("Control 尚未完成 owner 初始化")
	}
	return lease, nil
}

func (a *ControlAuthority) exchange(
	ctx context.Context,
	sessionToken string,
) (*Principal, State, error) {
	if principal, state, ok := a.cachedLease(sessionToken); ok {
		return principal, state, nil
	}
	var response controlExchangeResult
	err := a.call(ctx, http.MethodPost, "/internal/principals/exchange", map[string]string{
		"session_token": sessionToken,
		"audience":      a.verifier.audience,
	}, &response)
	if err != nil {
		return nil, State{}, mapControlError(err)
	}
	state := toControlState(response.State)
	if strings.TrimSpace(response.PrincipalToken) == "" {
		return nil, state, nil
	}
	claims, err := a.verifier.verify(response.PrincipalToken)
	if err != nil {
		return nil, State{}, err
	}
	controlValue := claims.principal()
	binding, err := a.bindings.resolve(ctx, controlValue)
	if err != nil {
		return nil, State{}, err
	}
	principal := projectControlPrincipal(controlValue, binding.LocalOwnerKey)
	a.storeLease(sessionToken, principal, state, time.Unix(claims.ExpiresAt, 0).UTC())
	return principal, state, nil
}

func (a *ControlAuthority) cachedLease(sessionToken string) (*Principal, State, bool) {
	key := hashSessionToken(sessionToken)
	if strings.TrimSpace(sessionToken) == "" {
		return nil, State{}, false
	}
	a.leaseMu.RLock()
	lease, ok := a.leases[key]
	a.leaseMu.RUnlock()
	if !ok {
		return nil, State{}, false
	}
	if !a.verifier.now().UTC().Before(lease.expiresAt) {
		a.leaseMu.Lock()
		delete(a.leases, key)
		a.leaseMu.Unlock()
		return nil, State{}, false
	}
	principal := *lease.principal
	return &principal, lease.state, true
}

func (a *ControlAuthority) storeLease(
	sessionToken string,
	principal *Principal,
	state State,
	expiresAt time.Time,
) {
	if strings.TrimSpace(sessionToken) == "" || principal == nil {
		return
	}
	now := a.verifier.now().UTC()
	a.leaseMu.Lock()
	defer a.leaseMu.Unlock()
	// ponytail: 首版登录或 refresh 时顺手清理，活跃会话明显增多后换成有界 LRU。
	for key, lease := range a.leases {
		if !now.Before(lease.expiresAt) {
			delete(a.leases, key)
		}
	}
	copy := *principal
	a.leases[hashSessionToken(sessionToken)] = controlCachedLease{
		principal: &copy,
		state:     state,
		expiresAt: expiresAt,
	}
}

func (a *ControlAuthority) deleteOwnerLeases(localOwnerKey string) {
	localOwnerKey = strings.TrimSpace(localOwnerKey)
	a.leaseMu.Lock()
	defer a.leaseMu.Unlock()
	for key, lease := range a.leases {
		if lease.principal != nil && lease.principal.UserID == localOwnerKey {
			delete(a.leases, key)
		}
	}
}

func (a *ControlAuthority) deleteSessionLeases(sessionID string) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}
	a.leaseMu.Lock()
	defer a.leaseMu.Unlock()
	for key, lease := range a.leases {
		if lease.principal == nil || lease.principal.SessionID == nil {
			continue
		}
		if strings.TrimSpace(*lease.principal.SessionID) == sessionID {
			delete(a.leases, key)
		}
	}
}

func (a *ControlAuthority) state(ctx context.Context) (State, error) {
	var response controlState
	if err := a.call(ctx, http.MethodGet, "/internal/state", nil, &response); err != nil {
		return State{}, mapControlError(err)
	}
	return toControlState(response), nil
}

func (a *ControlAuthority) call(
	ctx context.Context,
	method string,
	path string,
	input any,
	output any,
) error {
	if a == nil || a.baseURL == "" {
		return errors.New("NEXUS_CONTROL_URL 未配置")
	}
	if len(strings.TrimSpace(a.config.ControlServiceToken)) < 32 {
		return errors.New("NEXUS_CONTROL_SERVICE_TOKEN 至少需要 32 个字符")
	}
	endpoint, err := url.Parse(a.baseURL + controlAPIBase + path)
	if err != nil {
		return err
	}
	var body io.Reader
	if input != nil {
		payload, marshalErr := json.Marshal(input)
		if marshalErr != nil {
			return marshalErr
		}
		body = bytes.NewReader(payload)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(a.config.ControlServiceToken))
	request.Header.Set("Accept", "application/json")
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := a.client.Do(request)
	if err != nil {
		return fmt.Errorf("调用 nexus-control: %w", err)
	}
	defer response.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	var envelope controlEnvelope
	if err = decoder.Decode(&envelope); err != nil {
		return fmt.Errorf("解析 nexus-control 响应: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &controlRemoteError{Status: response.StatusCode, Code: envelope.Code, Message: envelope.Message}
	}
	if output == nil || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil
	}
	if err = json.Unmarshal(envelope.Data, output); err != nil {
		return fmt.Errorf("解析 nexus-control data: %w", err)
	}
	return nil
}

func projectControlPrincipal(value controlPrincipal, localOwnerKey string) *Principal {
	sessionID := strings.TrimSpace(value.SessionID)
	return &Principal{
		UserID:        strings.TrimSpace(localOwnerKey),
		ControlUserID: strings.TrimSpace(value.UserID),
		DeploymentID:  strings.TrimSpace(value.DeploymentID),
		Username:      strings.TrimSpace(value.Username),
		DisplayName:   strings.TrimSpace(value.DisplayName),
		Role:          strings.TrimSpace(value.Role),
		Avatar:        strings.TrimSpace(value.Avatar),
		AuthMethod:    strings.TrimSpace(value.AuthMethod),
		SessionID:     &sessionID,
	}
}

func toControlState(value controlState) State {
	return authctx.State{
		SetupRequired:        value.SetupRequired,
		SetupEnabled:         value.SetupEnabled,
		AuthRequired:         true,
		PasswordLoginEnabled: value.PasswordLoginEnabled,
	}
}

func mapControlError(err error) error {
	if err == nil {
		return nil
	}
	var remote *controlRemoteError
	if !errors.As(err, &remote) {
		return err
	}
	switch remote.Code {
	case "credentials_invalid":
		return ErrInvalidCredentials
	case "user_not_found":
		return ErrUserNotFound
	default:
		return err
	}
}
