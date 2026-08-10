// INPUT: Agent 更新时检测到的 runtime_version CAS 失败。
// OUTPUT: 服务层可稳定识别的 ErrRuntimeVersionConflict。
// POS: Agent 仓储乐观并发冲突的公共错误契约。
package agentrepo

import "errors"

// ErrRuntimeVersionConflict 表示 Agent runtime 已被其他写入更新。
var ErrRuntimeVersionConflict = errors.New("agent runtime version conflict")
