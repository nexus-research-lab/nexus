package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	authstore "github.com/nexus-research-lab/nexus/internal/storage/auth"
)

func TestPasswordChangeReceiptAndCredentialCASCommitTogether(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	db := handlertest.OpenSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	service := authsvc.NewServiceWithDB(cfg, db)
	user, err := service.InitOwner(ctx, authsvc.InitOwnerInput{
		Username: "admin",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("初始化 owner 失败: %v", err)
	}
	repository := authstore.NewRepository(cfg, db)
	_, original, err := repository.GetUserWithPasswordByID(ctx, user.UserID)
	if err != nil || original == nil {
		t.Fatalf("读取原密码凭据失败: credential=%+v err=%v", original, err)
	}

	firstHash, err := authsvc.HashPassword("password456")
	if err != nil {
		t.Fatalf("生成首个新 hash 失败: %v", err)
	}
	now := time.Now().UTC()
	first := authstore.PasswordCredential{
		CredentialID:      original.CredentialID,
		UserID:            user.UserID,
		PasswordHash:      firstHash,
		PasswordAlgo:      original.PasswordAlgo,
		PasswordUpdatedAt: now,
		CreatedAt:         original.CreatedAt,
		UpdatedAt:         now,
	}
	if replay, commitErr := repository.CommitPasswordChange(
		ctx,
		first,
		original.PasswordHash,
		"password-cas:first",
	); commitErr != nil || replay {
		t.Fatalf("首个改密事务应创建回执: replay=%v err=%v", replay, commitErr)
	}
	if outcome, settleErr := repository.SettlePasswordChangeNotApplied(
		ctx,
		user.UserID,
		"password-cas:first",
		now.Add(time.Second),
	); settleErr != nil || outcome != authstore.PasswordChangeOutcomeCommitted {
		t.Fatalf("committed 终态不得被放弃覆盖: outcome=%q err=%v", outcome, settleErr)
	}

	secondHash, err := authsvc.HashPassword("password789")
	if err != nil {
		t.Fatalf("生成第二个新 hash 失败: %v", err)
	}
	second := first
	second.PasswordHash = secondHash
	second.UpdatedAt = now.Add(time.Second)
	second.PasswordUpdatedAt = second.UpdatedAt
	if _, commitErr := repository.CommitPasswordChange(
		ctx,
		second,
		original.PasswordHash,
		"password-cas:stale",
	); !errors.Is(commitErr, authstore.ErrPasswordCredentialChanged) {
		t.Fatalf("过期旧 hash 必须被 CAS 拒绝: %v", commitErr)
	}
	outcome, err := repository.PasswordChangeOutcome(
		ctx,
		user.UserID,
		"password-cas:stale",
	)
	if err != nil || outcome != "" {
		t.Fatalf("被 CAS 拒绝的 request 不得伪造终态回执: outcome=%q err=%v", outcome, err)
	}

	outcome, err = repository.SettlePasswordChangeNotApplied(
		ctx,
		user.UserID,
		"password-cas:abandoned",
		now.Add(2*time.Second),
	)
	if err != nil || outcome != authstore.PasswordChangeOutcomeNotApplied {
		t.Fatalf("放弃请求必须持久化 not_applied: outcome=%q err=%v", outcome, err)
	}
	if _, commitErr := repository.CommitPasswordChange(
		ctx,
		second,
		first.PasswordHash,
		"password-cas:abandoned",
	); !errors.Is(commitErr, authstore.ErrPasswordChangeNotApplied) {
		t.Fatalf("已放弃 request 不得迟到修改凭据: %v", commitErr)
	}
}
