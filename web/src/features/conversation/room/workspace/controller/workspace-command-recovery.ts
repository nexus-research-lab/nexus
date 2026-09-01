/**
 * INPUT: 精确 Agent、workspace 命令参数，以及一次权威文件列表快照。
 * OUTPUT: 稳定的同意图锁键，或仅由当前文件状态证明的恢复结果。
 * POS: workspace 修改恢复的纯模型；不发送请求，也不把传输失败解释为未执行。
 */

import type {
  WorkspaceEntryMutationResponse,
  WorkspaceEntryRenameResponse,
  WorkspaceFileEntry,
} from "@/types/agent/agent";

export interface WorkspaceUploadFileIdentity {
  lastModified: number;
  name: string;
  size: number;
  type: string;
}

export type WorkspaceMutationIntent =
  | {
      agentId: string;
      command: "create";
      entryType: "directory" | "file";
      path: string;
    }
  | {
      agentId: string;
      command: "rename";
      isDirectory: boolean;
      newPath: string;
      path: string;
    }
  | {
      agentId: string;
      command: "delete";
      path: string;
    }
  | {
      agentId: string;
      command: "upload";
      file: WorkspaceUploadFileIdentity;
      targetDirectory: string | null;
    };

export type WorkspaceReconciledMutation =
  | {
      command: "create";
      entryType: "directory" | "file";
      result: WorkspaceEntryMutationResponse;
    }
  | {
      command: "rename";
      result: WorkspaceEntryRenameResponse;
    }
  | {
      command: "delete";
      result: WorkspaceEntryMutationResponse;
    };

export type WorkspaceUploadOutcomeStatus =
  | "completed"
  | "not_applied"
  | "not_started"
  | "unconfirmed";

export interface WorkspaceUploadOutcome {
  name: string;
  status: WorkspaceUploadOutcomeStatus;
}

function lockSegment(value: string): string {
  return `${value.length}:${value}`;
}

/**
 * 锁只覆盖同一 Agent 下的同一条副作用意图。长度前缀避免路径和分隔符碰撞；
 * 这个键只留在当前页面内，不进入业务协议、持久化或授权判断。
 */
export function getWorkspaceMutationIntentKey(
  intent: WorkspaceMutationIntent,
): string {
  const segments = [intent.agentId, intent.command];
  switch (intent.command) {
    case "create":
      segments.push(intent.entryType, intent.path);
      break;
    case "rename":
      segments.push(
        intent.isDirectory ? "directory" : "file",
        intent.path,
        intent.newPath,
      );
      break;
    case "delete":
      segments.push(intent.path);
      break;
    case "upload":
      segments.push(
        intent.targetDirectory ?? "",
        intent.file.name,
        String(intent.file.size),
        String(intent.file.lastModified),
        intent.file.type,
      );
      break;
  }
  return segments.map(lockSegment).join("|");
}

function containsWorkspacePath(
  files: WorkspaceFileEntry[],
  path: string,
): boolean {
  return files.some((entry) => (
    entry.path === path || entry.path.startsWith(`${path}/`)
  ));
}

/**
 * 当前列表只能证明“现在是否已达到目标状态”，不能证明历史请求由谁完成。
 * 上传可能被服务端改名或去重，列表又没有内容摘要，因此永不在这里猜测上传成功。
 */
export function reconcileWorkspaceMutation(
  intent: WorkspaceMutationIntent,
  files: WorkspaceFileEntry[],
): WorkspaceReconciledMutation | null {
  switch (intent.command) {
    case "create": {
      const target = files.find((entry) => entry.path === intent.path);
      if (!target || target.is_dir !== (intent.entryType === "directory")) {
        return null;
      }
      return {
        command: "create",
        entryType: intent.entryType,
        result: {path: target.path},
      };
    }
    case "rename": {
      const target = files.find((entry) => entry.path === intent.newPath);
      if (
        !target
        || target.is_dir !== intent.isDirectory
        || containsWorkspacePath(files, intent.path)
      ) {
        return null;
      }
      return {
        command: "rename",
        result: {path: intent.path, new_path: target.path},
      };
    }
    case "delete":
      return containsWorkspacePath(files, intent.path)
        ? null
        : {command: "delete", result: {path: intent.path}};
    case "upload":
      return null;
  }
}

export function groupWorkspaceUploadOutcomes(
  outcomes: WorkspaceUploadOutcome[],
): Record<WorkspaceUploadOutcomeStatus, string[]> {
  const grouped: Record<WorkspaceUploadOutcomeStatus, string[]> = {
    completed: [],
    not_applied: [],
    not_started: [],
    unconfirmed: [],
  };
  for (const outcome of outcomes) {
    grouped[outcome.status].push(outcome.name);
  }
  return grouped;
}
