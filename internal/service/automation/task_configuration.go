// INPUT: 标准化 scheduled task 创建意图。
// OUTPUT: 不含 request_id 的稳定 SHA-256 意图摘要。
// POS: 对话创建重试的幂等绑定，不参与任务业务配置。
package automation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

func taskCreateIntentDigest(input automationdomain.CreateJobInput) string {
	input.RequestID = ""
	payload, _ := json.Marshal(input)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
