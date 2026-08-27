// INPUT: completed Automation execution observation and its frozen delivery plan.
// OUTPUT: owner-confined immutable run artifact written before external delivery.
// POS: Execution/Delivery two-phase boundary; artifact delivery fields are pre-send facts.
package automation

import (
	"cmp"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func (s *Service) writeRunArtifact(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	runID string,
	roundID string,
	sessionKey string,
	finishedAt time.Time,
	status string,
	observation automationexec.ExecutionObservation,
	errorMessage *string,
	deliveryStatus string,
	deliveryError *string,
	deliveryTo string,
) *string {
	workspacePath, confinedRoot, err := s.openAutomationArtifactWorkspace(ctx, job)
	if err != nil {
		s.loggerFor(ctx).Warn("解析自动化任务运行产物目录失败", "job_id", job.JobID, "run_id", runID, "err", err)
		return nil
	}
	if strings.TrimSpace(workspacePath) == "" {
		return nil
	}
	defer confinedRoot.Close()
	relativePath := automationRunArtifactPath(job.JobID, runID)
	if err = confinedRoot.MkdirAll(
		filepath.Dir(relativePath),
		appfs.RuntimeCollaborativeDirectoryMode(0o755),
	); err != nil {
		s.loggerFor(ctx).Warn("创建自动化任务运行产物目录失败", "job_id", job.JobID, "run_id", runID, "err", err)
		return nil
	}
	content := renderRunArtifact(job, runID, roundID, sessionKey, finishedAt, status, observation, errorMessage, deliveryStatus, deliveryError, deliveryTo)
	if err = confinedRoot.WriteFileAtomic(
		relativePath,
		[]byte(content),
		appfs.RuntimeCollaborativeFileMode(0o644),
	); err != nil {
		s.loggerFor(ctx).Warn("写入自动化任务运行产物失败", "job_id", job.JobID, "run_id", runID, "err", err)
		return nil
	}
	return &relativePath
}

func (s *Service) openAutomationArtifactWorkspace(
	ctx context.Context,
	job automationdomain.ScheduledTask,
) (string, *confinedfs.Root, error) {
	if s.agents != nil && strings.TrimSpace(job.AgentID) != "" {
		agentValue, err := s.agents.GetAgent(ctx, strings.TrimSpace(job.AgentID))
		if err != nil {
			return "", nil, err
		}
		ownerUserID := strings.TrimSpace(job.OwnerUserID)
		if ownerUserID == "" || strings.TrimSpace(agentValue.OwnerUserID) != ownerUserID {
			return "", nil, fmt.Errorf("automation agent owner does not match job owner")
		}
		workspacePath := strings.TrimSpace(agentValue.WorkspacePath)
		root, err := workspacestore.New(s.config.WorkspacePath).OpenOwnerWorkspacePath(
			ownerUserID,
			workspacePath,
			true,
		)
		return workspacePath, root, err
	}
	workspacePath := strings.TrimSpace(s.config.WorkspacePath)
	ownerUserID := strings.TrimSpace(job.OwnerUserID)
	if workspacePath == "" || ownerUserID == "" {
		return "", nil, nil
	}
	root, err := workspacestore.New(s.config.WorkspacePath).OpenOwnerWorkspacePath(
		ownerUserID,
		workspacePath,
		true,
	)
	return workspacePath, root, err
}

func automationRunArtifactPath(jobID string, runID string) string {
	jobSegment := safeArtifactSegment(jobID, "job")
	runSegment := safeArtifactSegment(runID, "run")
	return filepath.ToSlash(filepath.Join(".nexus", "automation", "runs", jobSegment, runSegment+".md"))
}

func safeArtifactSegment(value string, fallback string) string {
	normalized := strings.TrimSpace(value)
	var builder strings.Builder
	for _, item := range normalized {
		if item >= 'a' && item <= 'z' || item >= 'A' && item <= 'Z' || item >= '0' && item <= '9' || item == '-' || item == '_' {
			builder.WriteRune(item)
		} else {
			builder.WriteRune('_')
		}
	}
	result := strings.Trim(builder.String(), "_")
	return cmp.Or(result, fallback)
}

func renderRunArtifact(
	job automationdomain.ScheduledTask,
	runID string,
	roundID string,
	sessionKey string,
	finishedAt time.Time,
	status string,
	observation automationexec.ExecutionObservation,
	errorMessage *string,
	deliveryStatus string,
	deliveryError *string,
	deliveryTo string,
) string {
	var builder strings.Builder
	builder.WriteString("# Automation Run\n\n")
	writeArtifactField(&builder, "Job", strings.TrimSpace(job.Name))
	writeArtifactField(&builder, "Job ID", strings.TrimSpace(job.JobID))
	writeArtifactField(&builder, "Run ID", strings.TrimSpace(runID))
	writeArtifactField(&builder, "Agent ID", strings.TrimSpace(job.AgentID))
	writeArtifactField(&builder, "Status", strings.TrimSpace(status))
	writeArtifactField(&builder, "Finished At", finishedAt.UTC().Format(time.RFC3339))
	writeArtifactField(&builder, "Session Key", strings.TrimSpace(sessionKey))
	writeArtifactField(&builder, "Round ID", strings.TrimSpace(roundID))
	writeArtifactField(&builder, "Runtime Session", anyStringPointer(observation.SessionID))
	writeArtifactField(&builder, "Message Count", fmt.Sprintf("%d", observation.MessageCount))
	writeArtifactField(&builder, "Delivery Status At Completion", strings.TrimSpace(deliveryStatus))
	writeArtifactField(&builder, "Frozen Delivery Target", strings.TrimSpace(deliveryTo))
	if errorMessage != nil {
		writeArtifactSection(&builder, "Error", *errorMessage)
	}
	if deliveryError != nil {
		writeArtifactSection(&builder, "Delivery Error", *deliveryError)
	}
	writeArtifactSection(&builder, "Result", observation.ResultText)
	writeArtifactSection(&builder, "Assistant", observation.AssistantText)
	return strings.TrimRight(builder.String(), "\n") + "\n"
}

func writeArtifactField(builder *strings.Builder, label string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	builder.WriteString("- ")
	builder.WriteString(label)
	builder.WriteString(": ")
	builder.WriteString(value)
	builder.WriteString("\n")
}

func writeArtifactSection(builder *strings.Builder, title string, content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}
	builder.WriteString("\n## ")
	builder.WriteString(title)
	builder.WriteString("\n\n")
	builder.WriteString(content)
	builder.WriteString("\n")
}
