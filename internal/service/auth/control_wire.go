package auth

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type controlState struct {
	SetupRequired        bool `json:"setup_required"`
	SetupEnabled         bool `json:"setup_enabled"`
	AuthRequired         bool `json:"auth_required"`
	PasswordLoginEnabled bool `json:"password_login_enabled"`
}

type controlPrincipal struct {
	DeploymentID string             `json:"deployment_id"`
	UserID       string             `json:"user_id"`
	Username     string             `json:"username"`
	DisplayName  string             `json:"display_name,omitempty"`
	Role         string             `json:"role"`
	Avatar       string             `json:"avatar,omitempty"`
	AuthMethod   string             `json:"auth_method"`
	SessionID    string             `json:"session_id"`
	Entitlement  controlEntitlement `json:"entitlement"`
}

type controlEntitlement struct {
	PlanKey           string    `json:"plan_key"`
	PlanName          string    `json:"plan_name"`
	MonthlyTokenLimit *int64    `json:"monthly_token_limit"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (p *controlPrincipal) normalize() {
	p.DeploymentID = strings.TrimSpace(p.DeploymentID)
	p.UserID = strings.TrimSpace(p.UserID)
	p.Username = strings.TrimSpace(p.Username)
	p.DisplayName = strings.TrimSpace(p.DisplayName)
	p.Role = strings.TrimSpace(p.Role)
	p.Avatar = strings.TrimSpace(p.Avatar)
	p.AuthMethod = strings.TrimSpace(p.AuthMethod)
	p.SessionID = strings.TrimSpace(p.SessionID)
	p.Entitlement.PlanKey = strings.TrimSpace(p.Entitlement.PlanKey)
	p.Entitlement.PlanName = strings.TrimSpace(p.Entitlement.PlanName)
	p.Entitlement.UpdatedAt = p.Entitlement.UpdatedAt.UTC()
}

type controlExchangeResult struct {
	PrincipalToken string       `json:"principal_token"`
	State          controlState `json:"state"`
}

// ControlIdentityInvalidation 是 Control 持久化身份变更序列中的一项。
type ControlIdentityInvalidation struct {
	EventID      int64     `json:"event_id"`
	DeploymentID string    `json:"deployment_id"`
	UserID       string    `json:"user_id"`
	SessionID    string    `json:"session_id,omitempty"`
	Reason       string    `json:"reason"`
	CreatedAt    time.Time `json:"created_at"`
}

type controlInvalidationBatch struct {
	Events     []ControlIdentityInvalidation `json:"events"`
	NextCursor int64                         `json:"next_cursor"`
}

type controlPrincipalClaims struct {
	Version      int                `json:"v"`
	Issuer       string             `json:"iss"`
	Audience     string             `json:"aud"`
	IssuedAt     int64              `json:"iat"`
	ExpiresAt    int64              `json:"exp"`
	DeploymentID string             `json:"deployment_id"`
	UserID       string             `json:"user_id"`
	Username     string             `json:"username"`
	DisplayName  string             `json:"display_name,omitempty"`
	Role         string             `json:"role"`
	Avatar       string             `json:"avatar,omitempty"`
	AuthMethod   string             `json:"auth_method"`
	SessionID    string             `json:"session_id"`
	Entitlement  controlEntitlement `json:"entitlement"`
}

func (c controlPrincipalClaims) principal() controlPrincipal {
	return controlPrincipal{
		DeploymentID: c.DeploymentID,
		UserID:       c.UserID,
		Username:     c.Username,
		DisplayName:  c.DisplayName,
		Role:         c.Role,
		Avatar:       c.Avatar,
		AuthMethod:   c.AuthMethod,
		SessionID:    c.SessionID,
		Entitlement:  c.Entitlement,
	}
}

type controlEnvelope struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type controlRemoteError struct {
	Status  int
	Code    string
	Message string
}

func (e *controlRemoteError) Error() string {
	if e == nil {
		return "Control 请求失败"
	}
	return fmt.Sprintf("Control 请求失败: %s (%s)", e.Message, e.Code)
}

type controlPrincipalVerifier struct {
	encodedKey string
	keyFile    string
	audience   string
	now        func() time.Time
	once       sync.Once
	publicKey  ed25519.PublicKey
	err        error
}

func (v *controlPrincipalVerifier) verify(token string) (controlPrincipalClaims, error) {
	if len(token) == 0 || len(token) > 16*1024 {
		return controlPrincipalClaims{}, errors.New("Control Principal token 长度无效")
	}
	v.once.Do(v.load)
	if v.err != nil {
		return controlPrincipalClaims{}, v.err
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return controlPrincipalClaims{}, errors.New("Control Principal token 格式无效")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return controlPrincipalClaims{}, errors.New("Control Principal header 无效")
	}
	var header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
		Version   int    `json:"v"`
	}
	if err = json.Unmarshal(headerBytes, &header); err != nil ||
		header.Algorithm != "EdDSA" ||
		header.Type != "NEXUS-PRINCIPAL" ||
		header.Version != 1 {
		return controlPrincipalClaims{}, errors.New("Control Principal header 不受支持")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !ed25519.Verify(v.publicKey, []byte(parts[0]+"."+parts[1]), signature) {
		return controlPrincipalClaims{}, errors.New("Control Principal 签名无效")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return controlPrincipalClaims{}, errors.New("Control Principal payload 无效")
	}
	var claims controlPrincipalClaims
	if err = json.Unmarshal(payload, &claims); err != nil {
		return controlPrincipalClaims{}, errors.New("Control Principal claims 无效")
	}
	now := v.now().UTC().Unix()
	if claims.Version != 1 ||
		claims.Issuer != "nexus-control" ||
		claims.Audience != v.audience ||
		claims.IssuedAt > now+30 ||
		claims.ExpiresAt <= now ||
		claims.ExpiresAt-claims.IssuedAt > 5*60 {
		return controlPrincipalClaims{}, errors.New("Control Principal claims 已过期或不匹配")
	}
	principal := claims.principal()
	principal.normalize()
	if principal.DeploymentID == "" ||
		principal.UserID == "" ||
		principal.Username == "" ||
		principal.SessionID == "" ||
		principal.AuthMethod != AuthMethodPassword ||
		principal.Entitlement.PlanKey == "" ||
		principal.Entitlement.PlanName == "" ||
		principal.Entitlement.UpdatedAt.IsZero() ||
		(principal.Entitlement.MonthlyTokenLimit != nil && *principal.Entitlement.MonthlyTokenLimit < 0) {
		return controlPrincipalClaims{}, errors.New("Control Principal 身份字段无效")
	}
	switch principal.Role {
	case RoleOwner, RoleAdmin, RoleMember:
	default:
		return controlPrincipalClaims{}, errors.New("Control Principal role 无效")
	}
	return claims, nil
}

func (v *controlPrincipalVerifier) load() {
	encoded := strings.TrimSpace(v.encodedKey)
	if encoded == "" {
		data, err := os.ReadFile(strings.TrimSpace(v.keyFile))
		if err != nil {
			v.err = fmt.Errorf("读取 Control Principal 公钥: %w", err)
			return
		}
		encoded = strings.TrimSpace(string(data))
	}
	key, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil || len(key) != ed25519.PublicKeySize {
		v.err = errors.New("Control Principal Ed25519 公钥无效")
		return
	}
	v.publicKey = ed25519.PublicKey(key)
}
