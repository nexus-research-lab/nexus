package automation

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

const (
	automationWriteRetryAttempts = 4
	automationWriteRetryDelay    = 50 * time.Millisecond
)

func (r *Repository) execWithRetry(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if r.isPostgres {
		return r.db.ExecContext(ctx, query, args...)
	}
	var lastErr error
	for attempt := 0; attempt < automationWriteRetryAttempts; attempt++ {
		result, err := r.db.ExecContext(ctx, query, args...)
		if err == nil {
			return result, nil
		}
		if !isSQLiteLockedError(err) || attempt == automationWriteRetryAttempts-1 {
			return nil, err
		}
		lastErr = err
		delay := automationWriteRetryDelay * time.Duration(attempt+1)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, lastErr
}

func isSQLiteLockedError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "database is locked")
}
