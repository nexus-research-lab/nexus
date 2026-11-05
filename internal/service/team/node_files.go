// INPUT: 原生 Room 生成的精确轮次交付凭据、owner 工作区读取入口与领取附带的群共享文件目录。
// OUTPUT: 本机持久 outbox 中的不可变文件内容；本轮可读的群共享文件本机副本。
// POS: 复用 deliver_files 与 Relay 文件目录的窄适配，不扫描目录或解析正文路径。
package team

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"slices"
	"time"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	relaycontract "github.com/nexus-research-lab/nexus/internal/relay"
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
		if err = o.executor.nodes.store.SaveNodeFiles(ctx, job, []teamstore.NodeFile{{Name: filepath.Base(block.Path), Data: data}}); err != nil {
			return err
		}
	}
	return nil
}

const (
	// 每次领取最多新下载 32 MiB；已同步且大小一致的不可变文件不重复下载。
	roomFilesDownloadBudget = 32 << 20
	roomFileMaxSize         = 20 << 20
)

// prepareRoomFiles 把群共享文件同步到本群会话公共附件目录 shared/<file_id>/，供本轮 Agent 读取。
// 共享文件按 ID 不可变；任何单个文件失败都只标记未同步，绝不让投递失败或重跑。
func (e *NodeExecutor) prepareRoomFiles(ctx context.Context, grant teamstore.NodeGrant, job teamstore.NodeJob, token string) []roomdomain.OnlineRoomFile {
	uploaders := make(map[string]string)
	for _, item := range job.Delivery.Messages {
		if item.AuthorType != "agent" && item.AuthorUserID != "" {
			uploaders[item.AuthorUserID] = item.AuthorDisplayName
		}
	}
	files := make([]roomdomain.OnlineRoomFile, 0, len(job.Delivery.RoomFiles))
	budget := int64(roomFilesDownloadBudget)
	for _, file := range job.Delivery.RoomFiles {
		entry := roomdomain.OnlineRoomFile{Name: file.Name, Size: file.Size, UploadedBy: file.UploaderID, UploadedAt: file.CreatedAt}
		if name := uploaders[file.UploaderID]; name != "" {
			entry.UploadedBy = name
		}
		entry.Path, entry.Skipped = e.syncRoomFile(ctx, job, token, file, &budget)
		if entry.Skipped != "" && entry.Skipped != "too_large" && entry.Skipped != "download_budget_exceeded" {
			e.logFailure(ctx, "sync_room_file", grant, job, errors.New(entry.Skipped))
		}
		files = append(files, entry)
	}
	return files
}

func (e *NodeExecutor) syncRoomFile(ctx context.Context, job teamstore.NodeJob, token string, file relaycontract.RoomFile, budget *int64) (string, string) {
	if e.assetFile == nil || e.upload == nil {
		return "", "unavailable"
	}
	name := filepath.Base(file.Name)
	if name != file.Name || name == "." || name == ".." {
		return "", "invalid_name"
	}
	destination := "shared/" + file.ID + "/"
	path, size, err := e.assetFile(ctx, job.RoomID, job.ConversationID, destination+name)
	if err != nil {
		return "", "unavailable"
	}
	if size == file.Size {
		return path, ""
	}
	if file.Size > roomFileMaxSize {
		return "", "too_large"
	}
	if file.Size > *budget {
		return "", "download_budget_exceeded"
	}
	// 与触发附件一致：每次真实下载前续租，避免连续下载耗尽 30 秒租约。
	if _, err = e.relay.RenewDelivery(ctx, token, job.Delivery.ID, job.Delivery.LeaseID, ""); err != nil {
		return "", "lease_renew_failed"
	}
	readCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	data, err := e.relay.DeliveryFile(readCtx, token, job.Delivery.ID, job.Delivery.LeaseID, relaycontract.MessageAttachment{ID: file.ID, Name: file.Name, Size: file.Size, SHA256: file.SHA256})
	cancel()
	if err != nil {
		return "", "download_failed"
	}
	*budget -= int64(len(data))
	result, err := e.upload(ctx, job.RoomID, job.ConversationID, name, destination, bytes.NewReader(data))
	if err != nil {
		return "", "write_failed"
	}
	if path, _, err = e.assetFile(ctx, job.RoomID, job.ConversationID, result.Path); err != nil {
		return "", "unavailable"
	}
	return path, ""
}
