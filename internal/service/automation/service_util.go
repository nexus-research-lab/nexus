package automation

import (
	"context"
	"strings"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

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

func anyString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, item := range values {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func contextForJobOwner(ctx context.Context, job automationdomain.CronJob) context.Context {
	ownerUserID := strings.TrimSpace(job.OwnerUserID)
	if ownerUserID == "" {
		return ctx
	}
	return authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID:     ownerUserID,
		Username:   ownerUserID,
		Role:       authctx.RoleOwner,
		AuthMethod: "automation",
	})
}

func backgroundContextForJobOwner(job automationdomain.CronJob) context.Context {
	return contextForJobOwner(context.Background(), job)
}

func stringPointer(value string) *string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return nil
	}
	return &normalized
}

func deliveryTargetSummary(target automationdomain.DeliveryTarget) string {
	mode := strings.TrimSpace(target.Mode)
	switch mode {
	case "", automationdomain.DeliveryModeNone:
		return ""
	case automationdomain.DeliveryModeLast:
		return automationdomain.DeliveryModeLast
	case automationdomain.DeliveryModeExplicit:
		parts := []string{automationdomain.DeliveryModeExplicit}
		if channel := strings.TrimSpace(target.Channel); channel != "" {
			parts = append(parts, channel)
		}
		if to := strings.TrimSpace(target.To); to != "" {
			parts = append(parts, to)
		}
		if threadID := strings.TrimSpace(target.ThreadID); threadID != "" {
			parts = append(parts, "thread:"+threadID)
		}
		return strings.Join(parts, ":")
	default:
		return mode
	}
}
