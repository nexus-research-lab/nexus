package auth

import (
	"context"
	"errors"
	"strings"

	authstore "github.com/nexus-research-lab/nexus/internal/storage/auth"
)

// InitOwner 初始化第一个 owner 用户。
func (s *Service) InitOwner(ctx context.Context, input InitOwnerInput) (*User, error) {
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
	if err = s.repository.CreateUserWithPassword(ctx, user, credential); err != nil {
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
	if strings.TrimSpace(input.CurrentPassword) == "" {
		return nil, errors.New("当前密码不能为空")
	}
	if err := validatePassword(input.NewPassword); err != nil {
		return nil, err
	}

	user, credential, err := s.repository.GetUserWithPasswordByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil || credential == nil || user.Status != UserStatusActive {
		return nil, ErrInvalidCredentials
	}
	matched, err := VerifyPassword(input.CurrentPassword, credential.PasswordHash)
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, ErrInvalidCredentials
	}

	passwordHash, err := HashPassword(input.NewPassword)
	if err != nil {
		return nil, err
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
	if err = s.repository.UpsertPasswordCredential(ctx, nextCredential); err != nil {
		return nil, err
	}
	return s.userByID(ctx, user.UserID)
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
