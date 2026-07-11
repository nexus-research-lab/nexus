package automation

import (
	"context"
	"strings"
	"time"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"

	"github.com/nexus-research-lab/nexus/internal/service/channels"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
)

const (
	maxAutoDeliveryAttempts = 5
	deliveryRetryBatchLimit = 20
)

type jobDeliveryResult struct {
	Status  string
	Error   *string
	Target  *channels.DeliveryTarget
	Receipt *channelmessage.Receipt
}

func toChannelDeliveryTarget(target automationdomain.DeliveryTarget) channels.DeliveryTarget {
	return channels.DeliveryTarget{
		Mode:      strings.TrimSpace(target.Mode),
		Channel:   strings.TrimSpace(target.Channel),
		To:        strings.TrimSpace(target.To),
		AccountID: strings.TrimSpace(target.AccountID),
		ThreadID:  strings.TrimSpace(target.ThreadID),
	}.Normalized()
}

var deliveryRetryBackoffs = []time.Duration{
	30 * time.Second,
	2 * time.Minute,
	10 * time.Minute,
	30 * time.Minute,
}

func (s *Service) deliverJobObservation(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	executionSessionKey string,
	observation automationexec.ExecutionObservation,
) jobDeliveryResult {
	deliveryMode := strings.TrimSpace(job.Delivery.Mode)
	deliveryChannel := strings.TrimSpace(job.Delivery.Channel)
	deliveryTo := strings.TrimSpace(job.Delivery.To)
	executionSessionKey = strings.TrimSpace(executionSessionKey)
	if deliveryMode == "" || deliveryMode == automationdomain.DeliveryModeNone {
		return jobDeliveryResult{Status: automationdomain.DeliveryStatusNotRequired}
	}
	if deliveryMode == automationdomain.DeliveryModeExplicit &&
		deliveryChannel == "websocket" &&
		deliveryTo != "" &&
		deliveryTo == executionSessionKey {
		return jobDeliveryResult{Status: automationdomain.DeliveryStatusSkipped}
	}
	if s.delivery == nil {
		return jobDeliveryResult{Status: automationdomain.DeliveryStatusFailed, Error: stringPointer("delivery router is not configured")}
	}
	text := firstNonEmpty(observation.ResultText, observation.AssistantText)
	if text == "" {
		return jobDeliveryResult{Status: automationdomain.DeliveryStatusSkipped}
	}
	target := toChannelDeliveryTarget(job.Delivery)
	if strings.TrimSpace(target.Mode) == channels.DeliveryModeLast {
		target.SessionKey = strings.TrimSpace(job.Source.SessionKey)
	}
	deliveryCtx := contextForJobOwner(ctx, job)
	delivered, err := s.delivery.DeliverMessage(
		deliveryCtx,
		job.AgentID,
		text,
		target,
	)
	if err != nil {
		return jobDeliveryResult{Status: automationdomain.DeliveryStatusFailed, Error: errorPointer(err)}
	}
	return jobDeliveryResult{Status: automationdomain.DeliveryStatusSucceeded, Target: &delivered.Target, Receipt: delivered.Receipt}
}

func (r jobDeliveryResult) deliveryTo(fallback automationdomain.DeliveryTarget) string {
	var summary string
	if r.Target != nil {
		summary = channelDeliveryTargetSummary(*r.Target)
	} else {
		summary = deliveryTargetSummary(fallback)
	}
	if r.Receipt == nil || strings.TrimSpace(r.Receipt.PrimaryPlatformMessageID) == "" {
		return summary
	}
	switch strings.TrimSpace(r.Receipt.Channel) {
	case "", channels.ChannelTypeInternal, channels.ChannelTypeWebSocket:
		return summary
	}
	messageID := strings.TrimSpace(r.Receipt.PrimaryPlatformMessageID)
	if summary == "" {
		return "message:" + messageID
	}
	return summary + ":message:" + messageID
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
	case automationdomain.DeliveryStatusSucceeded, automationdomain.DeliveryStatusFailed:
		return true
	default:
		return false
	}
}

func deliveredAtForStatus(status string, at time.Time) *time.Time {
	if strings.TrimSpace(status) != automationdomain.DeliveryStatusSucceeded {
		return nil
	}
	result := at.UTC()
	return &result
}

func deliveryRetrySchedule(status string, attemptsAfter int, now time.Time) (*time.Time, *time.Time) {
	if strings.TrimSpace(status) != automationdomain.DeliveryStatusFailed {
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
	configValue automationdomain.HeartbeatConfig,
	observation automationexec.ExecutionObservation,
) *string {
	targetMode := strings.TrimSpace(configValue.TargetMode)
	if targetMode == "" || targetMode == automationdomain.HeartbeatTargetNone {
		return nil
	}
	if s.delivery == nil {
		return stringPointer("delivery router is not configured")
	}
	filtered := automationexec.FilterHeartbeatResponse(
		firstNonEmpty(observation.ResultText, observation.AssistantText),
		configValue.AckMaxChars,
	)
	if !filtered.ShouldDeliver || strings.TrimSpace(filtered.Text) == "" {
		return nil
	}
	if _, err := s.delivery.DeliverMessage(
		context.Background(),
		agentID,
		filtered.Text,
		channels.DeliveryTarget{Mode: targetMode},
	); err != nil {
		return errorPointer(err)
	}
	return nil
}
