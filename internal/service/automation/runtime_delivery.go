package automation

import (
	"context"
	"strings"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation"
	"github.com/nexus-research-lab/nexus/internal/protocol"

	"github.com/nexus-research-lab/nexus/internal/service/channels"
)

const (
	maxAutoDeliveryAttempts = 5
	deliveryRetryBatchLimit = 20
)

type jobDeliveryResult struct {
	Status string
	Error  *string
	Target *channels.DeliveryTarget
}

var deliveryRetryBackoffs = []time.Duration{
	30 * time.Second,
	2 * time.Minute,
	10 * time.Minute,
	30 * time.Minute,
}

func (s *Service) deliverJobObservation(
	ctx context.Context,
	job protocol.CronJob,
	executionSessionKey string,
	observation automationdomain.ExecutionObservation,
) jobDeliveryResult {
	if strings.TrimSpace(job.Delivery.Mode) == "" || strings.TrimSpace(job.Delivery.Mode) == protocol.DeliveryModeNone {
		return jobDeliveryResult{Status: protocol.DeliveryStatusNotRequired}
	}
	if strings.TrimSpace(job.Delivery.Mode) == protocol.DeliveryModeExplicit &&
		strings.TrimSpace(job.Delivery.Channel) == "websocket" &&
		strings.TrimSpace(job.Delivery.To) != "" &&
		strings.TrimSpace(job.Delivery.To) == strings.TrimSpace(executionSessionKey) {
		return jobDeliveryResult{Status: protocol.DeliveryStatusSkipped}
	}
	if s.delivery == nil {
		return jobDeliveryResult{Status: protocol.DeliveryStatusFailed, Error: stringPointer("delivery router is not configured")}
	}
	text := firstNonEmpty(strings.TrimSpace(observation.ResultText), strings.TrimSpace(observation.AssistantText))
	if text == "" {
		return jobDeliveryResult{Status: protocol.DeliveryStatusSkipped}
	}
	deliveryCtx := contextForJobOwner(ctx, job)
	deliveredTarget, err := s.delivery.DeliverText(
		deliveryCtx,
		job.AgentID,
		text,
		toChannelDeliveryTarget(job.Delivery),
	)
	if err != nil {
		return jobDeliveryResult{Status: protocol.DeliveryStatusFailed, Error: errorPointer(err)}
	}
	return jobDeliveryResult{Status: protocol.DeliveryStatusSucceeded, Target: &deliveredTarget}
}

func (r jobDeliveryResult) deliveryTo(fallback protocol.DeliveryTarget) string {
	if r.Target != nil {
		return channelDeliveryTargetSummary(*r.Target)
	}
	return deliveryTargetSummary(fallback)
}

func channelDeliveryTargetSummary(target channels.DeliveryTarget) string {
	mode := strings.TrimSpace(target.Mode)
	switch mode {
	case "", channels.DeliveryModeNone:
		return ""
	case channels.DeliveryModeLast:
		return channels.DeliveryModeLast
	case channels.DeliveryModeExplicit:
		parts := []string{channels.DeliveryModeExplicit}
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

func deliveryAttempted(status string) bool {
	switch strings.TrimSpace(status) {
	case protocol.DeliveryStatusSucceeded, protocol.DeliveryStatusFailed:
		return true
	default:
		return false
	}
}

func deliveredAtForStatus(status string, at time.Time) *time.Time {
	if strings.TrimSpace(status) != protocol.DeliveryStatusSucceeded {
		return nil
	}
	result := at.UTC()
	return &result
}

func deliveryRetrySchedule(status string, attemptsAfter int, now time.Time) (*time.Time, *time.Time) {
	if strings.TrimSpace(status) != protocol.DeliveryStatusFailed {
		return nil, nil
	}
	if attemptsAfter >= maxAutoDeliveryAttempts {
		deadLetterAt := now.UTC()
		return nil, &deadLetterAt
	}
	index := attemptsAfter - 1
	if index < 0 || index >= len(deliveryRetryBackoffs) {
		deadLetterAt := now.UTC()
		return nil, &deadLetterAt
	}
	next := now.UTC().Add(deliveryRetryBackoffs[index])
	return &next, nil
}

func (s *Service) deliverHeartbeatObservation(
	agentID string,
	configValue protocol.HeartbeatConfig,
	observation automationdomain.ExecutionObservation,
) *string {
	if strings.TrimSpace(configValue.TargetMode) == "" || strings.TrimSpace(configValue.TargetMode) == protocol.HeartbeatTargetNone {
		return nil
	}
	if s.delivery == nil {
		return stringPointer("delivery router is not configured")
	}
	filtered := automationdomain.FilterHeartbeatResponse(
		firstNonEmpty(strings.TrimSpace(observation.ResultText), strings.TrimSpace(observation.AssistantText)),
		configValue.AckMaxChars,
	)
	if !filtered.ShouldDeliver || strings.TrimSpace(filtered.Text) == "" {
		return nil
	}
	if _, err := s.delivery.DeliverText(
		context.Background(),
		agentID,
		filtered.Text,
		channels.DeliveryTarget{Mode: strings.TrimSpace(configValue.TargetMode)},
	); err != nil {
		return errorPointer(err)
	}
	return nil
}
