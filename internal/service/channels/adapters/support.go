package adapters

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	channelmanagement "github.com/nexus-research-lab/nexus/internal/service/channels/management"
)

func AccountIDFromSecret(prefix string, secret string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(secret))
	return strings.TrimSpace(prefix) + "_" + hex.EncodeToString(sum[:8])
}

func IsPairingApprovalRequired(err error) bool {
	return channelmanagement.IsPairingApprovalRequired(err)
}

func PairingApprovalNoticeText(err error) string {
	return channelmanagement.PairingApprovalNoticeText(err)
}

func TruncateError(err error) string {
	if err == nil {
		return ""
	}
	text := strings.TrimSpace(err.Error())
	if text == "" {
		return "unknown error"
	}
	runes := []rune(text)
	if len(runes) <= 400 {
		return text
	}
	return string(runes[:400]) + "..."
}
