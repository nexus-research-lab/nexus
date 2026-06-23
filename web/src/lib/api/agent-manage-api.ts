/**
 * Agent API 服务模块
 *
 * [INPUT]: 依赖 @/types/agent/agent, @/types/system/api
 * [OUTPUT]: 对外提供 getAgents、createAgent、updateAgent、deleteAgent 等 API 函数
 * [POS]: lib 模块的 Agent API 层，被 agent store 消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

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
  apply_desktop_request_headers,
  is_desktop_runtime,
} from "@/config/desktop-runtime";
import { get_agent_api_base_url } from "@/config/options";
import { transform_api_agent } from "@/lib/api/agent-transform";
import { request_api } from "@/lib/api/http";

const AGENT_API_BASE_URL = get_agent_api_base_url();

// ==================== Agent API ====================

/** 获取所有 Agent 列表 */
export const get_agents = async (): Promise<Agent[]> => {
  const result = await request_api<ApiAgent[]>(`${AGENT_API_BASE_URL}/agents`, {
    method: "GET",
  });
  return result.map(transform_api_agent);
};

/** 创建 Agent */
export const create_agent_api = async (
  params: CreateAgentParams,
): Promise<Agent> => {
  const result = await request_api<ApiAgent>(`${AGENT_API_BASE_URL}/agents`, {
    method: "POST",
    body: JSON.stringify({
      name: params.name,
      options: params.options || null,
      avatar: params.avatar ?? null,
      description: params.description ?? null,
      vibe_tags: params.vibe_tags ?? [],
    }),
  });
  return transform_api_agent(result);
};

/** 更新 Agent */
export const update_agent_api = async (
  agent_id: string,
  params: UpdateAgentParams,
): Promise<Agent> => {
  const result = await request_api<ApiAgent>(
    `${AGENT_API_BASE_URL}/agents/${agent_id}`,
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
  return transform_api_agent(result);
};

/** 删除 Agent */
export const delete_agent_api = async (
  agent_id: string,
): Promise<{ success: boolean }> => {
  return request_api<{ success: boolean }>(
    `${AGENT_API_BASE_URL}/agents/${agent_id}`,
    {
      method: "DELETE",
    },
  );
};

/** 校验 Agent 名称 */
export const validate_agent_name_api = async (
  name: string,
  exclude_agent_id?: string,
): Promise<AgentNameValidationResult> => {
  const query = new URLSearchParams({ name });
  if (exclude_agent_id) {
    query.set("exclude_agent_id", exclude_agent_id);
  }

  return request_api<AgentNameValidationResult>(
    `${AGENT_API_BASE_URL}/agents/validate/name?${query.toString()}`,
    {
      method: "GET",
    },
  );
};

export const get_workspace_files_api = async (
  agent_id: string,
): Promise<WorkspaceFileEntry[]> => {
  return request_api<WorkspaceFileEntry[]>(
    `${AGENT_API_BASE_URL}/agents/${agent_id}/workspace/files`,
    {
      method: "GET",
    },
  );
};

export const get_workspace_file_content_api = async (
  agent_id: string,
  path: string,
): Promise<WorkspaceFileContent> => {
  const query = new URLSearchParams({ path });
  return request_api<WorkspaceFileContent>(
    `${AGENT_API_BASE_URL}/agents/${agent_id}/workspace/file?${query.toString()}`,
    {
      method: "GET",
    },
  );
};

export const update_workspace_file_content_api = async (
  agent_id: string,
  path: string,
  content: string,
): Promise<WorkspaceFileContent> => {
  return request_api<WorkspaceFileContent>(
    `${AGENT_API_BASE_URL}/agents/${agent_id}/workspace/file`,
    {
      method: "PUT",
      body: JSON.stringify({ path, content }),
    },
  );
};

export const create_workspace_entry_api = async (
  agent_id: string,
  path: string,
  entry_type: "file" | "directory",
  content: string = "",
): Promise<WorkspaceEntryMutationResponse> => {
  return request_api<WorkspaceEntryMutationResponse>(
    `${AGENT_API_BASE_URL}/agents/${agent_id}/workspace/entry`,
    {
      method: "POST",
      body: JSON.stringify({ path, entry_type, content }),
    },
  );
};

export const rename_workspace_entry_api = async (
  agent_id: string,
  path: string,
  new_path: string,
): Promise<WorkspaceEntryRenameResponse> => {
  return request_api<WorkspaceEntryRenameResponse>(
    `${AGENT_API_BASE_URL}/agents/${agent_id}/workspace/entry`,
    {
      method: "PATCH",
      body: JSON.stringify({ path, new_path }),
    },
  );
};

export const delete_workspace_entry_api = async (
  agent_id: string,
  path: string,
): Promise<WorkspaceEntryMutationResponse> => {
  const query = new URLSearchParams({ path });
  return request_api<WorkspaceEntryMutationResponse>(
    `${AGENT_API_BASE_URL}/agents/${agent_id}/workspace/entry?${query.toString()}`,
    {
      method: "DELETE",
    },
  );
};

/** 上传文件到 workspace */
export const upload_workspace_file_api = async (
  agent_id: string,
  file: File,
  path?: string,
): Promise<{ path: string; name: string; size: number }> => {
  const formData = new FormData();
  formData.append("file", file);
  if (path) {
    formData.append("path", path);
  }

  const result = await request_api<{
    path: string;
    name: string;
    size: number;
  }>(`${AGENT_API_BASE_URL}/agents/${agent_id}/workspace/upload`, {
    method: "POST",
    body: formData,
  });
  return result;
};

/** 获取 workspace 文件下载 URL */
function build_workspace_file_transfer_url(
  agent_id: string,
  path: string,
  disposition: "attachment" | "inline",
): string {
  const params = new URLSearchParams({ path, disposition });
  return `${AGENT_API_BASE_URL}/agents/${agent_id}/workspace/download?${params.toString()}`;
}

/** 获取 workspace 文件下载 URL */
const get_workspace_file_download_url = (
  agent_id: string,
  path: string,
): string => {
  return build_workspace_file_transfer_url(agent_id, path, "attachment");
};

/** 获取 workspace 文件预览 URL */
export const get_workspace_file_preview_url = (
  agent_id: string,
  path: string,
): string => {
  return build_workspace_file_transfer_url(agent_id, path, "inline");
};

const reveal_workspace_file_in_folder_api = async (
  agent_id: string,
  path: string,
): Promise<{ path: string }> => {
  return request_api<{ path: string }>(
    `${AGENT_API_BASE_URL}/agents/${agent_id}/workspace/reveal`,
    {
      method: "POST",
      body: { path },
    },
  );
};

function normalize_download_file_name(path: string, file_name?: string): string {
  const normalized_name = file_name?.trim();
  if (normalized_name) {
    return normalized_name;
  }
  const normalized_path = path.trim().replace(/\\/g, "/");
  return normalized_path.split("/").filter(Boolean).at(-1) || "download";
}

/** 桌面端在文件夹中定位，浏览器端下载文件，避免 HTML 触发桌面壳顶层导航。 */
export async function download_workspace_file_api(
  agent_id: string,
  path: string,
  file_name?: string,
): Promise<void> {
  if (is_desktop_runtime()) {
    await reveal_workspace_file_in_folder_api(agent_id, path);
    return;
  }

  const url = get_workspace_file_download_url(agent_id, path);
  const resolved_file_name = normalize_download_file_name(path, file_name);
  const headers = new Headers();
  apply_desktop_request_headers(url, headers);
  const response = await fetch(url, {
    credentials: "include",
    headers,
    method: "GET",
  });
  if (!response.ok) {
    throw new Error(`下载失败: ${response.status} ${response.statusText}`);
  }

  const blob = await response.blob();
  const object_url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = object_url;
  anchor.download = resolved_file_name;
  anchor.style.display = "none";
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  window.setTimeout(() => URL.revokeObjectURL(object_url), 0);
}
