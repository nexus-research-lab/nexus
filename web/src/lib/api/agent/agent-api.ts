/** Agent 与 workspace 的 HTTP 边界。 */

import {
  Agent,
  AgentNameValidationResult,
  ApiAgent,
  CreateAgentParams,
  UpdateAgentParams,
  WorkspaceFileContent,
  WorkspaceFileEntry,
  WorkspaceEntryMutationResponse,
  WorkspaceEntryRenameResponse,
} from "@/types/agent/agent";
import {
  applyDesktopRequestHeaders,
  isDesktopRuntime,
} from "@/config/desktop-runtime";
import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { transformApiAgent } from "@/lib/api/agent/agent-transform";
import { requestApi } from "@/lib/api/core/http";

const AGENT_API_BASE_URL = getAgentApiBaseUrl();

// ==================== Agent API ====================

/** 获取所有 Agent 列表 */
export const getAgents = async (): Promise<Agent[]> => {
  const result = await requestApi<ApiAgent[]>(`${AGENT_API_BASE_URL}/agents`, {
    method: "GET",
  });
  return result.map(transformApiAgent);
};

/** 创建 Agent */
export const createAgentApi = async (
  params: CreateAgentParams,
): Promise<Agent> => {
  const result = await requestApi<ApiAgent>(`${AGENT_API_BASE_URL}/agents`, {
    method: "POST",
    body: JSON.stringify({
      name: params.name,
      options: params.options || null,
      avatar: params.avatar ?? null,
      description: params.description ?? null,
      vibe_tags: params.vibe_tags ?? [],
    }),
  });
  return transformApiAgent(result);
};

/** 更新 Agent */
export const updateAgentApi = async (
  agentId: string,
  params: UpdateAgentParams,
): Promise<Agent> => {
  const result = await requestApi<ApiAgent>(
    `${AGENT_API_BASE_URL}/agents/${agentId}`,
    {
      method: "PATCH",
      body: JSON.stringify({
        name: params.name,
        options: params.options || null,
        avatar: params.avatar ?? null,
        description: params.description ?? null,
        vibe_tags: params.vibe_tags ?? [],
      }),
    },
  );
  return transformApiAgent(result);
};

/** 删除 Agent */
export const deleteAgentApi = async (
  agentId: string,
): Promise<{ success: boolean }> => {
  return requestApi<{ success: boolean }>(
    `${AGENT_API_BASE_URL}/agents/${agentId}`,
    {
      method: "DELETE",
    },
  );
};

/** 校验 Agent 名称 */
export const validateAgentNameApi = async (
  name: string,
  excludeAgentId?: string,
): Promise<AgentNameValidationResult> => {
  const query = new URLSearchParams({ name });
  if (excludeAgentId) {
    query.set("exclude_agent_id", excludeAgentId);
  }

  return requestApi<AgentNameValidationResult>(
    `${AGENT_API_BASE_URL}/agents/validate/name?${query.toString()}`,
    {
      method: "GET",
    },
  );
};

export const getWorkspaceFilesApi = async (
  agentId: string,
): Promise<WorkspaceFileEntry[]> => {
  return requestApi<WorkspaceFileEntry[]>(
    `${AGENT_API_BASE_URL}/agents/${agentId}/workspace/files`,
    {
      method: "GET",
    },
  );
};

export const getWorkspaceFileContentApi = async (
  agentId: string,
  path: string,
): Promise<WorkspaceFileContent> => {
  const query = new URLSearchParams({ path });
  return requestApi<WorkspaceFileContent>(
    `${AGENT_API_BASE_URL}/agents/${agentId}/workspace/file?${query.toString()}`,
    {
      method: "GET",
    },
  );
};

export const updateWorkspaceFileContentApi = async (
  agentId: string,
  path: string,
  content: string,
): Promise<WorkspaceFileContent> => {
  return requestApi<WorkspaceFileContent>(
    `${AGENT_API_BASE_URL}/agents/${agentId}/workspace/file`,
    {
      method: "PUT",
      body: JSON.stringify({ path, content }),
    },
  );
};

export const createWorkspaceEntryApi = async (
  agentId: string,
  path: string,
  entryType: "file" | "directory",
  content: string = "",
): Promise<WorkspaceEntryMutationResponse> => {
  return requestApi<WorkspaceEntryMutationResponse>(
    `${AGENT_API_BASE_URL}/agents/${agentId}/workspace/entry`,
    {
      method: "POST",
      body: JSON.stringify({ path, entry_type: entryType, content }),
    },
  );
};

export const renameWorkspaceEntryApi = async (
  agentId: string,
  path: string,
  newPath: string,
): Promise<WorkspaceEntryRenameResponse> => {
  return requestApi<WorkspaceEntryRenameResponse>(
    `${AGENT_API_BASE_URL}/agents/${agentId}/workspace/entry`,
    {
      method: "PATCH",
      body: JSON.stringify({ path, new_path: newPath }),
    },
  );
};

export const deleteWorkspaceEntryApi = async (
  agentId: string,
  path: string,
): Promise<WorkspaceEntryMutationResponse> => {
  const query = new URLSearchParams({ path });
  return requestApi<WorkspaceEntryMutationResponse>(
    `${AGENT_API_BASE_URL}/agents/${agentId}/workspace/entry?${query.toString()}`,
    {
      method: "DELETE",
    },
  );
};

/** 上传文件到 workspace */
export const uploadWorkspaceFileApi = async (
  agentId: string,
  file: File,
  path?: string,
): Promise<{ path: string; name: string; size: number }> => {
  const formData = new FormData();
  formData.append("file", file);
  if (path) {
    formData.append("path", path);
  }

  const result = await requestApi<{
    path: string;
    name: string;
    size: number;
  }>(`${AGENT_API_BASE_URL}/agents/${agentId}/workspace/upload`, {
    method: "POST",
    body: formData,
  });
  return result;
};

/** 获取 workspace 文件下载 URL */
function buildWorkspaceFileTransferUrl(
  agentId: string,
  path: string,
  disposition: "attachment" | "inline",
): string {
  const params = new URLSearchParams({ path, disposition });
  return `${AGENT_API_BASE_URL}/agents/${agentId}/workspace/download?${params.toString()}`;
}

/** 获取 workspace 文件下载 URL */
const getWorkspaceFileDownloadUrl = (
  agentId: string,
  path: string,
): string => {
  return buildWorkspaceFileTransferUrl(agentId, path, "attachment");
};

/** 获取 workspace 文件预览 URL */
export const getWorkspaceFilePreviewUrl = (
  agentId: string,
  path: string,
): string => {
  return buildWorkspaceFileTransferUrl(agentId, path, "inline");
};

const revealWorkspaceFileInFolderApi = async (
  agentId: string,
  path: string,
): Promise<{ path: string }> => {
  return requestApi<{ path: string }>(
    `${AGENT_API_BASE_URL}/agents/${agentId}/workspace/reveal`,
    {
      method: "POST",
      body: { path },
    },
  );
};

function normalizeDownloadFileName(path: string, fileName?: string): string {
  const normalizedName = fileName?.trim();
  if (normalizedName) {
    return normalizedName;
  }
  const normalizedPath = path.trim().replace(/\\/g, "/");
  return normalizedPath.split("/").filter(Boolean).at(-1) || "download";
}

/** 桌面端在文件夹中定位，浏览器端下载文件，避免 HTML 触发桌面壳顶层导航。 */
export async function downloadWorkspaceFileApi(
  agentId: string,
  path: string,
  fileName?: string,
): Promise<void> {
  if (isDesktopRuntime()) {
    await revealWorkspaceFileInFolderApi(agentId, path);
    return;
  }

  const url = getWorkspaceFileDownloadUrl(agentId, path);
  const resolvedFileName = normalizeDownloadFileName(path, fileName);
  const headers = new Headers();
  applyDesktopRequestHeaders(url, headers);
  const response = await fetch(url, {
    credentials: "include",
    headers,
    method: "GET",
  });
  if (!response.ok) {
    throw new Error(`下载失败: ${response.status} ${response.statusText}`);
  }

  const blob = await response.blob();
  const objectUrl = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = objectUrl;
  anchor.download = resolvedFileName;
  anchor.style.display = "none";
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  window.setTimeout(() => URL.revokeObjectURL(objectUrl), 0);
}
