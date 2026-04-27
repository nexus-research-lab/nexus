package automation

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func resolveSessionKey(job CronJob, runID *string) (string, error) {
	switch strings.TrimSpace(job.SessionTarget.Kind) {
	case SessionTargetMain:
		return buildMainSessionKey(job.AgentID), nil
	case SessionTargetBound:
		return strings.TrimSpace(job.SessionTarget.BoundSessionKey), nil
	case SessionTargetNamed:
		return protocol.BuildAgentSessionKey(job.AgentID, "automation", "dm", strings.TrimSpace(job.SessionTarget.NamedSessionKey), ""), nil
	default:
		if runID == nil || strings.TrimSpace(*runID) == "" {
			return "", errors.New("isolated target requires run_id")
		}
		return protocol.BuildAgentSessionKey(job.AgentID, "automation", "dm", fmt.Sprintf("cron:%s:%s", job.JobID, strings.TrimSpace(*runID)), ""), nil
	}
}

func buildMainSessionKey(agentID string) string {
	return protocol.BuildAgentSessionKey(strings.TrimSpace(agentID), "automation", "dm", "main", "")
}

func newAutomationID(prefix string) string {
	buffer := make([]byte, 10)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%s_%d", strings.TrimSpace(prefix), time.Now().UnixNano())
	}
	return strings.TrimSpace(prefix) + "_" + hex.EncodeToString(buffer)
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	result := value.UTC()
	return &result
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	result := strings.TrimSpace(*value)
	return &result
}

func errorPointer(err error) *string {
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(err.Error())
	return &message
}

func anyStringPointer(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func firstNonEmpty(values ...string) string {
	for _, item := range values {
		if strings.TrimSpace(item) != "" {
			return strings.TrimSpace(item)
		}
	}
	return ""
}

func stringPointer(value string) *string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return nil
	}
	return &normalized
}
