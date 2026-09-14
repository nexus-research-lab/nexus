// INPUT: 同一列表的文件产物记录与明确的来源工作区。
// OUTPUT: 按真实打开目标去重的稳定条目，保留最后一份记录和首次出现顺序。
// POS: 文件产物列表展示模型；不改历史记录，不按显示名或全局 Agent 推断身份。
import type { WorkspaceFileArtifactContent } from "@/types/conversation/message/content";
import { firstNonEmptyArtifactValue } from "./artifact-path-model";

export function buildWorkspaceFileArtifactEntries(
  artifacts: WorkspaceFileArtifactContent[],
  workspaceAgentId?: string | null,
): Array<{ key: string; artifact: WorkspaceFileArtifactContent }> {
  const entries = new Map<string, WorkspaceFileArtifactContent>();
  artifacts.forEach((artifact, index) => {
    const agentId = firstNonEmptyArtifactValue(artifact.workspace_agent_id, workspaceAgentId);
    const path = artifact.path.trim();
    // Unknown provenance cannot prove two records point to the same file.
    const key = agentId && path ? JSON.stringify([agentId, path]) : `unresolved:${index}`;
    entries.set(key, artifact);
  });
  return Array.from(entries, ([key, artifact]) => ({ key, artifact }));
}
