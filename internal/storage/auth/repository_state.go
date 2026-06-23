package auth

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

func (r *Repository) LoadState(ctx context.Context, accessTokenEnabled bool) (State, error) {
	state := State{}
	userCount, err := r.scalarCount(
		ctx,
		"SELECT COUNT(*) FROM users WHERE status = "+r.bind(1)+" AND user_id <> "+r.bind(2),
		UserStatusActive,
		authctx.SystemUserID,
	)
	if err != nil {
		return state, err
	}
	passwordUserCount, err := r.scalarCount(
		ctx,
		`SELECT COUNT(*)
FROM auth_password_credentials c
INNER JOIN users u ON u.user_id = c.user_id
WHERE u.status = `+r.bind(1)+` AND u.user_id <> `+r.bind(2),
		UserStatusActive,
		authctx.SystemUserID,
	)
	if err != nil {
		return state, err
	}
	state.UserCount = userCount
	state.PasswordUserCount = passwordUserCount
	state.SetupRequired = userCount == 0
	state.PasswordLoginEnabled = passwordUserCount > 0
	state.AccessTokenEnabled = accessTokenEnabled && userCount == 0
	state.AuthRequired = userCount > 0 || state.AccessTokenEnabled
	return state, nil
}

func (r *Repository) scalarCount(ctx context.Context, query string, args ...any) (int, error) {
	row := r.db.QueryRowContext(ctx, query, args...)
	var value int
	if err := row.Scan(&value); err != nil {
		return 0, err
	}
	return value, nil
}
