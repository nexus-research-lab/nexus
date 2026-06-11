package channels

import (
	"errors"
	"strings"
)

const pairingApprovalNoticeBody = "已收到你的消息，但当前 IM 用户或群聊尚未授权访问 Nexus 智能体。\n请管理员打开 Nexus 配对控制台（能力 → 配对），批准该配对后我就能继续回复。"

func pairingApprovalNoticeText(err error) string {
	if !isPairingApprovalRequired(err) {
		return ""
	}
	var approval *pairingApprovalError
	if errors.As(err, &approval) && strings.TrimSpace(approval.PairingID) != "" {
		return pairingApprovalNoticeBody + "\n配对 ID：" + strings.TrimSpace(approval.PairingID)
	}
	return pairingApprovalNoticeBody
}
