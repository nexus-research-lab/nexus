package automation

import (
	"context"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation"
	"github.com/nexus-research-lab/nexus/internal/protocol"

	"github.com/nexus-research-lab/nexus/internal/service/channels"
)

func (s *Service) deliverJobObservation(
	job protocol.CronJob,
	executionSessionKey string,
	observation automationdomain.ExecutionObservation,
) *string {
	if strings.TrimSpace(job.Delivery.Mode) == "" || strings.TrimSpace(job.Delivery.Mode) == protocol.DeliveryModeNone {
		return nil
	}
	if strings.TrimSpace(job.Delivery.Mode) == protocol.DeliveryModeExplicit &&
		strings.TrimSpace(job.Delivery.Channel) == "websocket" &&
		strings.TrimSpace(job.Delivery.To) != "" &&
		strings.TrimSpace(job.Delivery.To) == strings.TrimSpace(executionSessionKey) {
		return nil
	}
	if s.delivery == nil {
		return stringPointer("delivery router is not configured")
	}
	text := firstNonEmpty(strings.TrimSpace(observation.ResultText), strings.TrimSpace(observation.AssistantText))
	if text == "" {
		return nil
	}
	if _, err := s.delivery.DeliverText(
		context.Background(),
		job.AgentID,
		text,
		toChannelDeliveryTarget(job.Delivery),
	); err != nil {
		return errorPointer(err)
	}
	return nil
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
