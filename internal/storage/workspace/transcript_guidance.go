package workspace

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

var transcriptGuidanceLinePattern = regexp.MustCompile(`^\s*\d+\.\s+(?:round_id=([^:]+):\s*)?(.+?)\s*$`)

func buildTranscriptGuidanceMessages(
	sessionKey string,
	agentID string,
	currentRoundID string,
	entry map[string]any,
	timestamp int64,
) []protocol.Message {
	content := transcriptGuidanceAttachmentContent(entry)
	if content == "" {
		return nil
	}
	items := parseTranscriptGuidanceInputs(content)
	if len(items) == 0 {
		return nil
	}

	roundID := strings.TrimSpace(currentRoundID)
	if roundID == "" {
		roundID = buildTranscriptRoundID(firstNonEmpty(
			stringFromAny(entry["parentUuid"]),
			stringFromAny(entry["uuid"]),
		))
	}
	sessionID := stringFromAny(entry["session_id"])
	entryUUID := stringFromAny(entry["uuid"])
	rows := make([]protocol.Message, 0, len(items))
	for index, item := range items {
		sourceRoundID := strings.TrimSpace(item.RoundID)
		messageID := sourceRoundID
		if messageID == "" {
			messageID = firstNonEmpty(entryUUID, roundID) + ":guidance:" + strconv.Itoa(index+1)
		}
		rows = append(rows, message.NewGuidedInputMessage(message.GuidedInputMessageInput{
			MessageID:     messageID,
			SessionKey:    sessionKey,
			AgentID:       agentID,
			RoundID:       roundID,
			SourceRoundID: sourceRoundID,
			Content:       item.Content,
			SessionID:     sessionID,
			Timestamp:     timestamp + int64(index),
		}))
	}
	return rows
}

type transcriptGuidanceInput struct {
	RoundID string
	Content string
}

func transcriptGuidanceAttachmentContent(entry map[string]any) string {
	attachment, ok := entry["attachment"].(map[string]any)
	if !ok {
		return ""
	}
	if stringFromAny(attachment["type"]) != "hook_additional_context" {
		return ""
	}

	content := strings.TrimSpace(joinTranscriptGuidanceContent(attachment["content"]))
	if !strings.Contains(content, "<nexus_guidance>") {
		return ""
	}
	return content
}

func joinTranscriptGuidanceContent(value any) string {
	if text := stringFromAny(value); text != "" {
		return text
	}
	items, ok := value.([]any)
	if !ok {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		if text := stringFromAny(item); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func parseTranscriptGuidanceInputs(content string) []transcriptGuidanceInput {
	lines := strings.Split(content, "\n")
	items := make([]transcriptGuidanceInput, 0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "<nexus_guidance>" || line == "</nexus_guidance>" {
			continue
		}
		matches := transcriptGuidanceLinePattern.FindStringSubmatch(line)
		if len(matches) != 3 {
			continue
		}
		itemContent := strings.TrimSpace(matches[2])
		if itemContent == "" {
			continue
		}
		items = append(items, transcriptGuidanceInput{
			RoundID: strings.TrimSpace(matches[1]),
			Content: itemContent,
		})
	}
	return items
}
