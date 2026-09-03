package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"

	_ "modernc.org/sqlite"
)

func TestControlAuthorityVerifiesPrincipalAndBindsLocalOwner(t *testing.T) {
	t.Parallel()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	claims := controlPrincipalClaims{
		Version: 1, Issuer: "nexus-control", Audience: "nexus-runtime",
		IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix(),
		DeploymentID: "dep-a", UserID: "user-control-a",
		Username: "admin", DisplayName: "Admin", Role: RoleOwner,
		AuthMethod: AuthMethodPassword, SessionID: "sess-a",
	}
	token := signControlTestPrincipal(t, privateKey, claims)
	const serviceToken = "control-service-token-32-characters"
	var exchanges atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+serviceToken {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		if request.URL.Path != controlAPIBase+"/internal/principals/exchange" {
			http.NotFound(writer, request)
			return
		}
		exchanges.Add(1)
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"code": "0000",
			"data": map[string]any{
				"principal_token": token,
				"state": map[string]any{
					"auth_required": true, "password_login_enabled": true,
					"setup_enabled": true,
				},
			},
		})
	}))
	t.Cleanup(server.Close)

	cfg, database := newAuthTestDB(t)
	cfg.ControlURL = server.URL
	cfg.ControlServiceToken = serviceToken
	cfg.ControlPrincipalPublicKey = base64.RawStdEncoding.EncodeToString(publicKey)
	cfg.ControlPrincipalAudience = "nexus-runtime"
	cfg.ControlRequestTimeoutSeconds = 2
	authority := NewControlAuthority(cfg, database, nil)
	request := httptest.NewRequest(http.MethodGet, "/nexus/v1/agents", nil)
	request.AddCookie(&http.Cookie{Name: "nexus_session", Value: "opaque-session"})
	principal, state, err := authority.InspectRequest(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if principal == nil ||
		principal.UserID == "user-control-a" ||
		!strings.HasPrefix(principal.UserID, "owner_") ||
		principal.ControlUserID != "user-control-a" ||
		principal.DeploymentID != "dep-a" ||
		!state.AuthRequired {
		t.Fatalf("principal = %+v, state = %+v", principal, state)
	}
	var localOwnerKey string
	if err = database.QueryRow(
		`SELECT local_owner_key FROM local_owner_bindings
WHERE deployment_id = ? AND control_user_id = ?`,
		"dep-a",
		"user-control-a",
	).Scan(&localOwnerKey); err != nil {
		t.Fatal(err)
	}
	if localOwnerKey != principal.UserID {
		t.Fatalf("local owner key = %q", localOwnerKey)
	}
	if _, _, err = authority.InspectRequest(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if exchanges.Load() != 1 {
		t.Fatalf("Control exchanges = %d, want 1 within signed lease", exchanges.Load())
	}
	status, err := authority.BuildStatusPayload(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if status.UserID == nil || *status.UserID != "user-control-a" {
		t.Fatalf("public user id = %v", status.UserID)
	}
	if !status.SetupEnabled {
		t.Fatalf("status = %+v", status)
	}
	server.Close()
	authority.verifier.now = func() time.Time { return now.Add(2 * time.Minute) }
	if _, _, err = authority.InspectRequest(context.Background(), request); err == nil {
		t.Fatal("expired Control lease must fail closed when Control is unavailable")
	}
}

func TestControlBindingCreateClaimsOneOwnerAcrossStores(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg, database := newAuthTestDB(t)
	open := func() *sql.DB {
		db, err := sql.Open(
			"sqlite",
			cfg.DatabaseURL+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_txlock=immediate",
		)
		if err != nil {
			t.Fatal(err)
		}
		db.SetMaxOpenConns(1)
		t.Cleanup(func() { _ = db.Close() })
		return db
	}
	stores := []*controlBindingStore{
		newControlBindingStore("sqlite", open()),
		newControlBindingStore("sqlite", open()),
	}
	principal := controlPrincipal{
		DeploymentID: "dep-atomic", UserID: "user-atomic",
		Username: "atomic", DisplayName: "Atomic", Role: RoleMember,
	}
	bindings := []controlOwnerBinding{
		{DeploymentID: principal.DeploymentID, ControlUserID: principal.UserID, LocalOwnerKey: deterministicControlOwnerKey(principal.DeploymentID, principal.UserID)},
		{DeploymentID: principal.DeploymentID, ControlUserID: principal.UserID, LocalOwnerKey: deterministicControlOwnerKey(principal.DeploymentID, principal.UserID)},
	}
	type result struct {
		claimed bool
		err     error
	}
	start := make(chan struct{})
	results := make(chan result, len(stores))
	for index := range stores {
		go func(index int) {
			<-start
			claimed, err := stores[index].create(ctx, bindings[index], principal)
			results <- result{claimed: claimed, err: err}
		}(index)
	}
	close(start)
	claims := 0
	for range stores {
		value := <-results
		if value.err != nil {
			t.Fatal(value.err)
		}
		if value.claimed {
			claims++
		}
	}
	if claims != 1 {
		t.Fatalf("binding claims = %d, want 1", claims)
	}
	var bindingCount, ownerCount int
	if err := database.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM local_owner_bindings WHERE deployment_id = ? AND control_user_id = ?`,
		principal.DeploymentID,
		principal.UserID,
	).Scan(&bindingCount); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM owner_profiles WHERE owner_user_id = ?`,
		bindings[0].LocalOwnerKey,
	).Scan(&ownerCount); err != nil {
		t.Fatal(err)
	}
	if bindingCount != 1 || ownerCount != 1 {
		t.Fatalf("binding count = %d, owner projections = %d", bindingCount, ownerCount)
	}
}

func TestControlIdentityInvalidationClearsBoundLease(t *testing.T) {
	t.Parallel()
	const serviceToken = "control-service-token-32-characters"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+serviceToken {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		data := any(nil)
		switch request.URL.Path {
		case controlAPIBase + "/internal/identity-invalidations/latest":
			data = map[string]any{"cursor": 7}
		case controlAPIBase + "/internal/identity-invalidations":
			if request.URL.Query().Get("after") != "7" || request.URL.Query().Get("limit") != "256" {
				http.Error(writer, "bad query", http.StatusBadRequest)
				return
			}
			data = map[string]any{
				"events": []map[string]any{{
					"event_id": 8, "deployment_id": "dep-a", "user_id": "user-a",
					"reason": "principal_changed", "created_at": time.Now().UTC(),
				}},
				"next_cursor": 8,
			}
		case controlAPIBase + "/internal/users/user-a/role":
			data = map[string]string{"role": RoleAdmin}
		default:
			http.NotFound(writer, request)
			return
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"code": "0000", "data": data})
	}))
	t.Cleanup(server.Close)
	cfg, database := newAuthTestDB(t)
	cfg.ControlURL = server.URL
	cfg.ControlServiceToken = serviceToken
	authority := NewControlAuthority(cfg, database, nil)
	principal := controlPrincipal{
		DeploymentID: "dep-a", UserID: "user-a", Username: "member",
		DisplayName: "Member", Role: RoleMember,
	}
	binding, err := authority.bindings.resolve(context.Background(), principal)
	if err != nil {
		t.Fatal(err)
	}
	authority.storeLease(
		"session-a",
		projectControlPrincipal(principal, binding.LocalOwnerKey),
		State{AuthRequired: true},
		time.Now().UTC().Add(time.Minute),
	)
	cursor, err := authority.LatestControlIdentityInvalidationID(context.Background())
	if err != nil || cursor != 7 {
		t.Fatalf("cursor = %d, err = %v", cursor, err)
	}
	events, err := authority.ControlIdentityInvalidations(context.Background(), cursor)
	if err != nil || len(events) != 1 {
		t.Fatalf("events = %+v, err = %v", events, err)
	}
	owner, err := authority.ApplyControlIdentityInvalidation(context.Background(), events[0])
	if err != nil || owner != binding.LocalOwnerKey {
		t.Fatalf("owner = %q, err = %v", owner, err)
	}
	if _, _, ok := authority.cachedLease("session-a"); ok {
		t.Fatal("identity invalidation left cached lease active")
	}
	var role, status string
	if err = database.QueryRow(
		`SELECT role, status FROM owner_profiles WHERE owner_user_id = ?`,
		binding.LocalOwnerKey,
	).Scan(&role, &status); err != nil {
		t.Fatal(err)
	}
	if role != RoleAdmin || status != UserStatusActive {
		t.Fatalf("owner projection role = %q, status = %q", role, status)
	}
}

func TestControlSessionInvalidationClearsOnlyExactLease(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg, database := newAuthTestDB(t)
	authority := NewControlAuthority(cfg, database, nil)
	controlValue := controlPrincipal{
		DeploymentID: "dep-a", UserID: "user-a", Username: "member",
		DisplayName: "Member", Role: RoleMember, AuthMethod: AuthMethodPassword,
	}
	binding, err := authority.bindings.resolve(ctx, controlValue)
	if err != nil {
		t.Fatal(err)
	}
	expiresAt := time.Now().UTC().Add(time.Minute)
	for _, sessionID := range []string{"session-a", "session-b"} {
		principal := controlValue
		principal.SessionID = sessionID
		authority.storeLease(
			"cookie-"+sessionID,
			projectControlPrincipal(principal, binding.LocalOwnerKey),
			State{AuthRequired: true},
			expiresAt,
		)
	}
	owner, err := authority.ApplyControlIdentityInvalidation(ctx, ControlIdentityInvalidation{
		EventID: 1, DeploymentID: "dep-a", UserID: "user-a",
		SessionID: "session-a", Reason: "session_revoked",
	})
	if err != nil || owner != binding.LocalOwnerKey {
		t.Fatalf("owner = %q, err = %v", owner, err)
	}
	if _, _, ok := authority.cachedLease("cookie-session-a"); ok {
		t.Fatal("revoked session lease remains cached")
	}
	if _, _, ok := authority.cachedLease("cookie-session-b"); !ok {
		t.Fatal("sibling session lease was cleared")
	}
}

func TestControlBindingPreservesImportedOwnerKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg, database := newAuthTestDB(t)
	now := time.Now().UTC()
	if _, err := database.ExecContext(ctx, `INSERT INTO owner_profiles (
owner_user_id, username, display_name, role, status, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?)`, "user_existing", "admin", "Admin", RoleOwner, UserStatusActive, now, now); err != nil {
		t.Fatal(err)
	}
	store := newControlBindingStore(cfg.DatabaseDriver, database)
	binding, err := store.resolve(ctx, controlPrincipal{
		DeploymentID: "dep-a", UserID: "user_existing", Username: "admin",
		DisplayName: "Admin", Role: RoleOwner, AuthMethod: AuthMethodPassword,
	})
	if err != nil {
		t.Fatal(err)
	}
	if binding.LocalOwnerKey != "user_existing" {
		t.Fatalf("local owner key = %q", binding.LocalOwnerKey)
	}
}

func signControlTestPrincipal(
	t *testing.T,
	privateKey ed25519.PrivateKey,
	claims controlPrincipalClaims,
) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString(
		[]byte(`{"alg":"EdDSA","typ":"NEXUS-PRINCIPAL","v":1}`),
	)
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	signed := strings.Join([]string{header, body}, ".")
	signature := ed25519.Sign(privateKey, []byte(signed))
	return signed + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func newAuthTestDB(t *testing.T) (config.Config, *sql.DB) {
	t.Helper()
	cfg := config.Config{
		APIPrefix:      "/nexus/v1",
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(t.TempDir(), "auth.db"),
	}
	handlertest.MigrateSQLiteFromDir(t, cfg.DatabaseURL, authMigrationDir(t))
	database, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return cfg, database
}

func authMigrationDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位 auth 测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}
