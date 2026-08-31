/**
 * INPUT: Agent 领域参数、workspace 路径/正文与可选读取 revision。
 * OUTPUT: Agent HTTP 资源、owner-scoped 创建回执与可条件提交的 workspace 文件内容。
 * POS: Agent/workspace HTTP 边界；创建对账只使用领域 request ID，不复用 HTTP 诊断 ID。
 */

import {
  Agent,
  AgentCreationRequestResult,
  AgentContact,
  AgentProfileTemplateResponse,
  ApiAgent,
  ApiAgentCreationRequestResult,
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
      profile_template: params.profile_template ?? null,
      vibe_tags: params.vibe_tags ?? [],
      creation_request_id: params.creation_request_id ?? null,
    }),
  });
  return transformApiAgent(result);
};

/** 按当前 owner 与 exact 领域请求 ID 对账，不会触发创建。 */
export const getAgentCreationRequestApi = async (
  creationRequestId: string,
): Promise<AgentCreationRequestResult> => {
  const result = await requestApi<ApiAgentCreationRequestResult>(
    `${AGENT_API_BASE_URL}/agents/create-requests/${encodeURIComponent(creationRequestId)}`,
    { method: "GET" },
  );
  return {
    agent: result.agent ? transformApiAgent(result.agent) : null,
    creationRequestId: result.creation_request_id,
    status: result.status,
  };
};

/** 获取创建 Agent 时的默认行为模板。 */
export const getAgentProfileTemplateApi =
  async (): Promise<AgentProfileTemplateResponse> => {
    return requestApi<AgentProfileTemplateResponse>(
      `${AGENT_API_BASE_URL}/agents/profile-template`,
      { method: "GET" },
    );
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

/** 获取指定普通 Agent 的好友通讯录。 */
export const listAgentContactsApi = async (
  agentId: string,
): Promise<AgentContact[]> => {
  return requestApi<AgentContact[]>(
    `${AGENT_API_BASE_URL}/agents/${encodeURIComponent(agentId)}/contacts`,
    { method: "GET" },
  );
};

/** 创建双向好友关系，或更新当前 Agent 为好友设置的别名。 */
export const upsertAgentContactApi = async (
  agentId: string,
  contactAgentId: string,
  alias: string,
): Promise<AgentContact> => {
  return requestApi<AgentContact>(
    `${AGENT_API_BASE_URL}/agents/${encodeURIComponent(agentId)}/contacts`,
    {
      method: "POST",
      body: JSON.stringify({
        contact_agent_id: contactAgentId,
        alias: alias.trim(),
      }),
    },
  );
};

/** 删除双方的好友关系，保留已有消息历史。 */
export const deleteAgentContactApi = async (
  agentId: string,
  contactAgentId: string,
): Promise<{ success: boolean }> => {
  return requestApi<{ success: boolean }>(
    `${AGENT_API_BASE_URL}/agents/${encodeURIComponent(agentId)}/contacts/${encodeURIComponent(contactAgentId)}`,
    { method: "DELETE" },
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
  expectedRevision?: string,
): Promise<WorkspaceFileContent> => {
  return requestApi<WorkspaceFileContent>(
    `${AGENT_API_BASE_URL}/agents/${agentId}/workspace/file`,
    {
      method: "PUT",
      body: JSON.stringify({
        path,
        content,
        ...(expectedRevision ? { expected_revision: expectedRevision } : {}),
      }),
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

/** 将已有 workspace 文件读取为 Composer 可直接复用的浏览器 File。 */
export async function loadWorkspaceFileApi(
  agentId: string,
  path: string,
  fileName?: string,
): Promise<File> {
  const url = getWorkspaceFileDownloadUrl(agentId, path);
  const headers = new Headers();
  applyDesktopRequestHeaders(url, headers);
  const response = await fetch(url, {
    credentials: "include",
    headers,
    method: "GET",
  });
  if (!response.ok) {
    throw new Error(`读取文件失败: ${response.status} ${response.statusText}`);
  }
  const blob = await response.blob();
  return new File([blob], normalizeDownloadFileName(path, fileName), {
    lastModified: Date.now(),
    type: blob.type,
  });
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

  const file = await loadWorkspaceFileApi(agentId, path, fileName);
  const objectUrl = URL.createObjectURL(file);
  const anchor = document.createElement("a");
  anchor.href = objectUrl;
  anchor.download = file.name;
  anchor.style.display = "none";
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  window.setTimeout(() => URL.revokeObjectURL(objectUrl), 0);
}
