package llm

import (
	"encoding/json"
	"errors"
	"strings"

	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
)

func parseTextResponse(apiFormat string, body []byte) (string, error) {
	switch normalizeAPIFormat(apiFormat) {
	case providersvc.APIFormatResponses:
		var payload responsesResponse
		if err := json.Unmarshal(body, &payload); err != nil {
			return "", err
		}
		if text := payload.firstText(); text != "" {
			return text, nil
		}
		return "", errors.New("llm responses response missing text")
	case providersvc.APIFormatChatCompletions:
		var payload chatCompletionsResponse
		if err := json.Unmarshal(body, &payload); err != nil {
			return "", err
		}
		return payload.firstText(), nil
	default:
		var payload anthropicMessagesResponse
		if err := json.Unmarshal(body, &payload); err != nil {
			return "", err
		}
		return payload.firstText(), nil
	}
}

type anthropicMessagesResponse struct {
	Content []anthropicContentBlock `json:"content"`
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (r anthropicMessagesResponse) firstText() string {
	for _, item := range r.Content {
		if strings.TrimSpace(item.Type) == "text" && strings.TrimSpace(item.Text) != "" {
			return item.Text
		}
	}
	return ""
}

type chatCompletionsResponse struct {
	Choices []chatChoice `json:"choices"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
	Text    string      `json:"text"`
}

type chatMessage struct {
	Content string `json:"content"`
}

func (r chatCompletionsResponse) firstText() string {
	for _, choice := range r.Choices {
		if strings.TrimSpace(choice.Message.Content) != "" {
			return choice.Message.Content
		}
		if strings.TrimSpace(choice.Text) != "" {
			return choice.Text
		}
	}
	return ""
}

type responsesResponse struct {
	OutputText string           `json:"output_text"`
	Output     []responsesItem  `json:"output"`
	Content    []responsesBlock `json:"content"`
}

type responsesItem struct {
	Type    string           `json:"type"`
	Content []responsesBlock `json:"content"`
}

type responsesBlock struct {
	Type       string `json:"type"`
	Text       string `json:"text"`
	OutputText string `json:"output_text"`
}

func (r responsesResponse) firstText() string {
	if strings.TrimSpace(r.OutputText) != "" {
		return r.OutputText
	}
	for _, item := range r.Output {
		for _, block := range item.Content {
			if strings.TrimSpace(block.Text) != "" {
				return block.Text
			}
			if strings.TrimSpace(block.OutputText) != "" {
				return block.OutputText
			}
		}
	}
	for _, block := range r.Content {
		if strings.TrimSpace(block.Text) != "" {
			return block.Text
		}
		if strings.TrimSpace(block.OutputText) != "" {
			return block.OutputText
		}
	}
	return ""
}
