// INPUT: 原生 Room 生成的精确轮次交付凭据与 owner 工作区读取入口。
// OUTPUT: 本机持久 outbox 中的不可变文件内容，不扫描目录或解析正文路径。
// POS: 复用 deliver_files 与 Relay 文件目录的窄适配。
package team

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"slices"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	teamstore "github.com/nexus-research-lab/nexus/internal/storage/teamrelay"
)

func (o *nodeObserver) captureFiles(ctx context.Context, job *teamstore.NodeJob, event protocol.EventMessage) error {
	raw, err := json.Marshal(event.Data["content"])
	if err != nil {
		return err
	}
	var blocks []protocol.WorkspaceFileArtifactBlock
	// 非块状文本消息没有文件凭据。
	if json.Unmarshal(raw, &blocks) != nil {
		return nil
	}
	for _, block := range blocks {
		if block.Type != protocol.ContentBlockTypeWorkspaceFileArtifact || block.Role != "deliverable" || block.ID == "" || slices.Contains(job.ArtifactIDs, block.ID) {
			continue
		}
		agentRoundID := event.AgentRoundID
		if agentRoundID == "" {
			agentRoundID, _ = event.Data["agent_round_id"].(string)
		}
		if block.WorkspaceAgentID != job.LocalAgentID || block.ProducerAgentID != job.LocalAgentID || block.Scope != protocol.WorkspaceFileArtifactScopeAgentWorkspace || agentRoundID == "" || block.SourceAgentRoundID != agentRoundID {
			return errors.New("产物来源不属于当前执行 Agent")
		}
		if o.executor.openFile == nil || len(job.ArtifactIDs) >= 32 {
			return errors.New("在线产物交付不可用或超过 32 项")
		}
		file, _, err := o.executor.openFile(ctx, job.LocalAgentID, block.Path)
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(io.LimitReader(file, (20<<20)+1))
		_ = file.Close()
		if readErr != nil {
			return readErr
		}
		if len(data) > 20<<20 || job.ArtifactBytes+int64(len(data)) > 32<<20 {
			return errors.New("在线产物超过单文件 20 MiB 或任务合计 32 MiB")
		}
		job.ArtifactIDs = append(job.ArtifactIDs, block.ID)
		job.ArtifactBytes += int64(len(data))
		// ponytail: 单任务最多 32 MiB，冻结字节复用 SQLite outbox；更大产物再引入本机 blob 存储。
		if err = o.executor.nodes.store.SaveNodeFiles(ctx, *job, []teamstore.NodeFile{{Name: filepath.Base(block.Path), Data: data}}); err != nil {
			return err
		}
		job.Sequence++
	}
	return nil
}
