package tool

import (
	"errors"
	"time"

	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/argx"
)

func parseExpiresAt(args map[string]any) (*time.Time, error) {
	raw, ok := args["expires_at"]
	if !ok {
		return nil, nil
	}
	value := argx.StringOf(raw)
	if value == "" {
		return nil, errors.New("expires_at cannot be empty; use clear_expires_at to remove it")
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, errors.New("expires_at must be an RFC3339 timestamp")
	}
	utc := parsed.UTC()
	return &utc, nil
}
