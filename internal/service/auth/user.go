// INPUT: Owner 初始化/用户管理请求、认证存储与 runtime 安全转场协调器。
// OUTPUT: 经校验的认证用户；server 首个 owner 仅在 pre-auth runtime 撤销后提交。
// POS: 认证用户生命周期与首次启用认证的事务入口。
package auth

import (
	"context"
	"errors"
	"strings"

	authstore "github.com/nexus-research-lab/nexus/internal/storage/auth"
)

// InitOwner 初始化第一个 owner 用户。
func (s *Service) InitOwner(ctx context.Context, input InitOwnerInput) (*User, error) {
	s.initOwnerMu.Lock()
	defer s.initOwnerMu.Unlock()

	state, err := s.GetState(ctx)
	if err != nil {
		return nil, err
	}
	if state.UserCount > 0 {
		return nil, ErrOwnerAlreadyInitialized
	}

	username, err := normalizeUsername(input.Username)
	if err != nil {
		return nil, err
	}
	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		displayName = username
	}
	if err = validatePassword(input.Password); err != nil {
		return nil, err
	}
	passwordHash, err := HashPassword(input.Password)
	if err != nil {
		return nil, err
	}

	now := s.now()
	user := authstore.UserRecord{
		UserID:      s.idFactory("user"),
		Username:    username,
		DisplayName: displayName,
		Role:        RoleOwner,
		Status:      UserStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	credential := authstore.PasswordCredential{
		CredentialID:      s.idFactory("cred"),
		UserID:            user.UserID,
		PasswordHash:      passwordHash,
		PasswordAlgo:      passwordAlgorithmArgon2ID,
		PasswordUpdatedAt: now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	commit := func(commitContext context.Context) error {
		return s.repository.CreateUserWithPassword(commitContext, user, credential)
	}
	if s.runtimeTransition != nil && !s.desktopAuthBypassEnabled() {
		err = s.runtimeTransition.EnableAuthentication(ctx, commit)
	} else {
		err = commit(ctx)
	}
	if err != nil {
		return nil, err
	}
	return s.userByID(ctx, user.UserID)
}

// CreateUser 创建新的认证用户。
func (s *Service) CreateUser(ctx context.Context, input CreateUserInput) (*User, error) {
	username, err := normalizeUsername(input.Username)
	if err != nil {
		return nil, err
	}
	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		displayName = username
	}
	if err = validatePassword(input.Password); err != nil {
		return nil, err
	}
	role, err := normalizeUserRole(input.Role)
	if err != nil {
		return nil, err
	}
	existing, err := s.repository.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrUsernameAlreadyExists
	}

	passwordHash, err := HashPassword(input.Password)
	if err != nil {
		return nil, err
	}
	now := s.now()
	user := authstore.UserRecord{
		UserID:      s.idFactory("user"),
		Username:    username,
		DisplayName: displayName,
		Role:        role,
		Status:      UserStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	credential := authstore.PasswordCredential{
		CredentialID:      s.idFactory("cred"),
		UserID:            user.UserID,
		PasswordHash:      passwordHash,
		PasswordAlgo:      passwordAlgorithmArgon2ID,
		PasswordUpdatedAt: now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err = s.repository.CreateUserWithPassword(ctx, user, credential); err != nil {
		return nil, err
	}
	return s.userByID(ctx, user.UserID)
}

// ListUsers 列出当前全部用户。
func (s *Service) ListUsers(ctx context.Context) ([]User, error) {
	records, err := s.repository.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	users := make([]User, 0, len(records))
	for _, record := range records {
		users = append(users, toUser(record))
	}
	return users, nil
}

// ResetPassword 重置指定用户密码。
func (s *Service) ResetPassword(ctx context.Context, input ResetPasswordInput) (*User, error) {
	if err := validatePassword(input.Password); err != nil {
		return nil, err
	}

	var (
		user *authstore.UserRecord
		err  error
	)
	if strings.TrimSpace(input.UserID) != "" {
		user, err = s.repository.GetUserByID(ctx, input.UserID)
	} else if strings.TrimSpace(input.Username) != "" {
		user, err = s.repository.GetUserByUsername(ctx, strings.TrimSpace(input.Username))
	} else {
		return nil, errors.New("user_id 与 username 至少提供一个")
	}
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	passwordHash, err := HashPassword(input.Password)
	if err != nil {
		return nil, err
	}
	now := s.now()
	credential := authstore.PasswordCredential{
		CredentialID:      s.idFactory("cred"),
		UserID:            user.UserID,
		PasswordHash:      passwordHash,
		PasswordAlgo:      passwordAlgorithmArgon2ID,
		PasswordUpdatedAt: now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err = s.repository.UpsertPasswordCredential(ctx, credential); err != nil {
		return nil, err
	}
	return s.userByID(ctx, user.UserID)
}

// ChangePassword 校验当前密码后修改当前用户密码。
func (s *Service) ChangePassword(ctx context.Context, input ChangePasswordInput) (*User, error) {
	userID := strings.TrimSpace(input.UserID)
	if userID == "" {
		return nil, errors.New("user_id 不能为空")
	}
	requestID, err := normalizePasswordChangeRequestID(input.RequestID)
	if err != nil {
		return nil, errors.Join(ErrPasswordChangeInvalidInput, err)
	}
	outcome, err := s.repository.PasswordChangeOutcome(ctx, userID, requestID)
	if err != nil {
		return nil, err
	}
	switch outcome {
	case authstore.PasswordChangeOutcomeCommitted:
		return s.passwordChangeCommittedUser(ctx, userID)
	case authstore.PasswordChangeOutcomeNotApplied:
		return nil, ErrPasswordChangeNotApplied
	}
	if strings.TrimSpace(input.CurrentPassword) == "" {
		return s.rejectPasswordChange(
			ctx,
			userID,
			requestID,
			errors.Join(ErrPasswordChangeInvalidInput, errors.New("当前密码不能为空")),
			true,
		)
	}
	if err := validatePassword(input.NewPassword); err != nil {
		return s.rejectPasswordChange(
			ctx,
			userID,
			requestID,
			errors.Join(ErrPasswordChangeInvalidInput, err),
			true,
		)
	}

	user, credential, err := s.repository.GetUserWithPasswordByID(ctx, userID)
	if err != nil {
		return s.rejectPasswordChange(ctx, userID, requestID, err, false)
	}
	if user == nil || credential == nil || user.Status != UserStatusActive {
		return s.rejectPasswordChange(ctx, userID, requestID, ErrInvalidCredentials, true)
	}
	matched, err := VerifyPassword(input.CurrentPassword, credential.PasswordHash)
	if err != nil {
		return s.rejectPasswordChange(ctx, userID, requestID, err, false)
	}
	if !matched {
		return s.rejectPasswordChange(ctx, userID, requestID, ErrInvalidCredentials, true)
	}

	passwordHash, err := HashPassword(input.NewPassword)
	if err != nil {
		return s.rejectPasswordChange(ctx, userID, requestID, err, false)
	}
	now := s.now()
	nextCredential := authstore.PasswordCredential{
		CredentialID:      s.idFactory("cred"),
		UserID:            user.UserID,
		PasswordHash:      passwordHash,
		PasswordAlgo:      passwordAlgorithmArgon2ID,
		PasswordUpdatedAt: now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if _, err = s.repository.CommitPasswordChange(
		ctx,
		nextCredential,
		credential.PasswordHash,
		requestID,
	); err != nil {
		if errors.Is(err, authstore.ErrPasswordCredentialChanged) {
			return s.rejectPasswordChange(ctx, userID, requestID, ErrInvalidCredentials, true)
		}
		if errors.Is(err, authstore.ErrPasswordChangeNotApplied) {
			return nil, ErrPasswordChangeNotApplied
		}
		if authstore.IsPasswordChangeOutcomeUnknown(err) {
			return nil, err
		}
		return nil, err
	}
	updatedUser, err := s.userByID(ctx, user.UserID)
	if err != nil {
		return nil, errors.Join(ErrPasswordChangeCommitted, err)
	}
	return updatedUser, nil
}

// PasswordChangeOutcome 返回 exact user/request 的 durable 终态；无回执时为 unknown。
func (s *Service) PasswordChangeOutcome(
	ctx context.Context,
	userID string,
	requestID string,
) (PasswordChangeOutcome, error) {
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return "", errors.New("user_id 不能为空")
	}
	normalizedRequestID, err := normalizePasswordChangeRequestID(requestID)
	if err != nil {
		return "", errors.Join(ErrPasswordChangeInvalidInput, err)
	}
	outcome, err := s.repository.PasswordChangeOutcome(ctx, normalizedUserID, normalizedRequestID)
	if err != nil {
		return "", err
	}
	return projectPasswordChangeOutcome(outcome), nil
}

// SettlePasswordChangeNotApplied 原子放弃尚未提交的 exact request，并阻止迟到写入。
func (s *Service) SettlePasswordChangeNotApplied(
	ctx context.Context,
	userID string,
	requestID string,
) (PasswordChangeOutcome, error) {
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return "", errors.New("user_id 不能为空")
	}
	normalizedRequestID, err := normalizePasswordChangeRequestID(requestID)
	if err != nil {
		return "", errors.Join(ErrPasswordChangeInvalidInput, err)
	}
	outcome, err := s.repository.SettlePasswordChangeNotApplied(
		ctx,
		normalizedUserID,
		normalizedRequestID,
		s.now(),
	)
	if err != nil {
		return "", err
	}
	return projectPasswordChangeOutcome(outcome), nil
}

func (s *Service) rejectPasswordChange(
	ctx context.Context,
	userID string,
	requestID string,
	rejection error,
	clientSafe bool,
) (*User, error) {
	outcome, err := s.repository.SettlePasswordChangeNotApplied(
		ctx,
		userID,
		requestID,
		s.now(),
	)
	if err != nil {
		return nil, err
	}
	if outcome == authstore.PasswordChangeOutcomeCommitted {
		return s.passwordChangeCommittedUser(ctx, userID)
	}
	if clientSafe {
		return nil, rejection
	}
	return nil, errors.Join(ErrPasswordChangeNotApplied, rejection)
}

func (s *Service) passwordChangeCommittedUser(ctx context.Context, userID string) (*User, error) {
	user, err := s.userByID(ctx, userID)
	if err != nil {
		return nil, errors.Join(ErrPasswordChangeCommitted, err)
	}
	return user, nil
}

func projectPasswordChangeOutcome(outcome authstore.PasswordChangeOutcome) PasswordChangeOutcome {
	switch outcome {
	case authstore.PasswordChangeOutcomeCommitted:
		return PasswordChangeOutcomeCommitted
	case authstore.PasswordChangeOutcomeNotApplied:
		return PasswordChangeOutcomeNotApplied
	default:
		return PasswordChangeOutcomeUnknown
	}
}

// UpdateProfile 更新当前用户的个人资料。
func (s *Service) UpdateProfile(ctx context.Context, input UpdateProfileInput) (*User, error) {
	userID := strings.TrimSpace(input.UserID)
	if userID == "" {
		return nil, errors.New("user_id 不能为空")
	}
	if userID == SystemUserID && s.desktopAuthBypassEnabled() {
		return s.updateDesktopLocalProfile(ctx, input)
	}

	user, err := s.repository.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil || user.Status != UserStatusActive {
		return nil, ErrUserNotFound
	}

	if input.Avatar != nil {
		avatar, avatarErr := normalizeAvatar(*input.Avatar)
		if avatarErr != nil {
			return nil, avatarErr
		}
		if err = s.repository.UpdateUserAvatar(ctx, userID, avatar, s.now()); err != nil {
			return nil, err
		}
	}
	return s.userByID(ctx, userID)
}

func (s *Service) userByID(ctx context.Context, userID string) (*User, error) {
	record, err := s.repository.GetUserByID(ctx, userID)
	if err != nil || record == nil {
		return nil, err
	}
	user := toUser(*record)
	return &user, nil
}

func toUser(record authstore.UserRecord) User {
	return User{
		UserID:      record.UserID,
		Username:    record.Username,
		DisplayName: record.DisplayName,
		Role:        record.Role,
		Status:      record.Status,
		Avatar:      record.Avatar,
		LastLoginAt: record.LastLoginAt,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}
}
