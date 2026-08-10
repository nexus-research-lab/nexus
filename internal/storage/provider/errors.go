// INPUT: Provider/model 条件写入的 RowsAffected 与持久化版本。
// OUTPUT: 服务层可稳定识别的不存在与 configuration_version 冲突错误。
// POS: Provider 仓储 CAS 的错误协议。
package provider

import "errors"

var (
	// ErrProviderNotFound 表示条件写入的 Provider 已不存在。
	ErrProviderNotFound = errors.New("provider not found")
	// ErrConfigurationVersionConflict 表示 Provider 聚合已被其他写入推进。
	ErrConfigurationVersionConflict = errors.New("provider configuration version conflict")
	// ErrModelNotFound 表示条件写入的模型卡不存在。
	ErrModelNotFound = errors.New("provider model not found")
)
