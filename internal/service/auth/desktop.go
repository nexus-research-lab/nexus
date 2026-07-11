package auth

import (
	"context"
	"strings"

	authstore "github.com/nexus-research-lab/nexus/internal/storage/auth"
)

const (
	localDesktopUsername    = "local"
	localDesktopDisplayName = "Local User"
)

func (s *Service) desktopAuthBypassEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(s.config.AppMode), "desktop")
}

func (s *Service) desktopLocalPrincipal(ctx context.Context) (*Principal, error) {
	user, err := s.localDesktopUser(ctx)
	if err != nil {
		return nil, err
	}
	return &Principal{
		UserID:      user.UserID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		Avatar:      user.Avatar,
		AuthMethod:  AuthMethodLocal,
	}, nil
}

func (s *Service) localDesktopUser(ctx context.Context) (*User, error) {
	now := s.now()
	user := User{
		UserID:      SystemUserID,
		Username:    localDesktopUsername,
		DisplayName: localDesktopDisplayName,
		Role:        RoleOwner,
		Status:      UserStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	record, err := s.repository.GetUserByID(ctx, SystemUserID)
	if err != nil || record == nil {
		return &user, err
	}
	if record.Status != UserStatusActive {
		return &user, nil
	}
	user.Avatar = strings.TrimSpace(record.Avatar)
	user.CreatedAt = record.CreatedAt
	user.UpdatedAt = record.UpdatedAt
	if displayName := strings.TrimSpace(record.DisplayName); displayName != "" {
		user.DisplayName = displayName
	}
	return &user, nil
}

func (s *Service) updateDesktopLocalProfile(ctx context.Context, input UpdateProfileInput) (*User, error) {
	user, err := s.localDesktopUser(ctx)
	if err != nil {
		return nil, err
	}
	if input.Avatar != nil {
		avatar, avatarErr := normalizeAvatar(*input.Avatar)
		if avatarErr != nil {
			return nil, avatarErr
		}
		user.Avatar = avatar
	}
	now := s.now()
	createdAt := user.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}
	if err = s.repository.UpsertLocalUser(ctx, authstore.UserRecord{
		UserID:      SystemUserID,
		Username:    SystemUserID,
		DisplayName: localDesktopDisplayName,
		Role:        RoleOwner,
		Status:      UserStatusActive,
		Avatar:      user.Avatar,
		CreatedAt:   createdAt,
		UpdatedAt:   now,
	}); err != nil {
		return nil, err
	}
	return s.localDesktopUser(ctx)
}
