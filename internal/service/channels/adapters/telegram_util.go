package adapters

import (
	"errors"
	"strings"
)

func (c *TelegramChannel) redactError(err error) error {
	if err == nil {
		return nil
	}
	text := strings.TrimSpace(err.Error())
	token := strings.TrimSpace(c.token)
	if token != "" {
		text = strings.ReplaceAll(text, "/bot"+token+"/", "/bot<redacted>/")
		text = strings.ReplaceAll(text, "bot"+token, "bot<redacted>")
	}
	if text == "" {
		text = "telegram request failed"
	}
	return errors.New(text)
}
