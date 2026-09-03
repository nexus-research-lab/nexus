/**
 * INPUT: Agent 领域参数、workspace 路径/正文与可选读取 revision。
 * OUTPUT: Agent HTTP 资源、owner-scoped 创建回执、可条件提交正文与有界 Range 文件片段。
 * POS: Agent/workspace HTTP 边界；大文件不组装为 JSON/Blob，创建对账不复用 HTTP 诊断 ID。
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
  WorkspaceFileTextChunk,
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
const WORKSPACE_TEXT_CHUNK_BYTES = 512 * 1024;
const WORKSPACE_TEXT_CHUNK_LOOKAHEAD_BYTES = 3;

export class WorkspaceFileSizeLimitError extends Error {
  constructor() {
    super("workspace file exceeds the requested size limit");
    this.name = "WorkspaceFileSizeLimitError";
  }
}

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
      business_tags: params.business_tags ?? [],
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
        business_tags: params.business_tags,
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

/** 大型文本只读取一个固定字节片段，避免整个文件进入 WebView 内存。 */
export async function getWorkspaceFileTextChunkApi(
  agentId: string,
  path: string,
  offset: number,
  signal: AbortSignal,
): Promise<WorkspaceFileTextChunk> {
  if (!Number.isSafeInteger(offset) || offset < 0) {
    throw new Error("invalid workspace file chunk offset");
  }
  const url = getWorkspaceFileDownloadUrl(agentId, path);
  const headers = new Headers({
    Range: `bytes=${offset}-${offset + WORKSPACE_TEXT_CHUNK_BYTES + WORKSPACE_TEXT_CHUNK_LOOKAHEAD_BYTES - 1}`,
  });
  applyDesktopRequestHeaders(url, headers);
  const response = await fetch(url, {
    credentials: "include",
    headers,
    method: "GET",
    signal,
  });
  const contentRange = parseWorkspaceContentRange(response.headers.get("content-range"));
  const contentLength = finiteHeaderNumber(response.headers.get("content-length"));
  if (
    response.status !== 206
    || !contentRange
    || contentRange.start !== offset
    || contentLength === null
    || contentLength < 0
    || contentLength > WORKSPACE_TEXT_CHUNK_BYTES + WORKSPACE_TEXT_CHUNK_LOOKAHEAD_BYTES
    || contentRange.end - contentRange.start + 1 !== contentLength
  ) {
    await response.body?.cancel();
    throw new Error(`读取文件片段失败: HTTP ${response.status}`);
  }
  const bytes = new Uint8Array(await response.arrayBuffer());
  if (bytes.length !== contentLength) {
    throw new Error("文件片段响应不完整");
  }
  const visibleLength = utf8ChunkPrefixLength(bytes);
  if (visibleLength === 0 && contentRange.size > offset) {
    throw new Error("文件不是有效的 UTF-8 文本");
  }
  const content = new TextDecoder("utf-8", { fatal: true }).decode(
    bytes.subarray(0, visibleLength),
  );
  const nextOffset = offset + visibleLength;
  return {
    content,
    nextOffset: nextOffset < contentRange.size ? nextOffset : null,
    offset,
    size: contentRange.size,
  };
}

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
  fileName: string | undefined,
  maxBytes: number,
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
  const responseSize = workspaceTransferSize(response.headers);
  if (responseSize === null || responseSize > maxBytes) {
    await response.body?.cancel();
    throw new WorkspaceFileSizeLimitError();
  }
  const blob = await response.blob();
  if (blob.size > maxBytes) {
    throw new WorkspaceFileSizeLimitError();
  }
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

  const anchor = document.createElement("a");
  anchor.href = getWorkspaceFileDownloadUrl(agentId, path);
  anchor.download = normalizeDownloadFileName(path, fileName);
  anchor.style.display = "none";
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
}

function workspaceTransferSize(headers: Headers): number | null {
  const contentRange = parseWorkspaceContentRange(headers.get("content-range"));
  if (contentRange) {
    return contentRange.size;
  }
  return finiteHeaderNumber(headers.get("content-length"));
}

function finiteHeaderNumber(value: string | null): number | null {
  if (value === null || value.trim() === "") {
    return null;
  }
  const number = Number(value);
  return Number.isSafeInteger(number) && number >= 0 ? number : null;
}

function parseWorkspaceContentRange(value: string | null): {
  end: number;
  size: number;
  start: number;
} | null {
  const match = /^bytes (\d+)-(\d+)\/(\d+)$/.exec(value?.trim() ?? "");
  if (!match) {
    return null;
  }
  const start = Number(match[1]);
  const end = Number(match[2]);
  const size = Number(match[3]);
  return Number.isSafeInteger(start)
    && Number.isSafeInteger(end)
    && Number.isSafeInteger(size)
    && start >= 0
    && end >= start
    && size > end
    ? { end, size, start }
    : null;
}

function utf8ChunkPrefixLength(bytes: Uint8Array): number {
  const preferredLength = Math.min(bytes.length, WORKSPACE_TEXT_CHUNK_BYTES);
  const minimumLength = Math.max(0, preferredLength - WORKSPACE_TEXT_CHUNK_LOOKAHEAD_BYTES);
  for (let length = preferredLength; length >= minimumLength; length -= 1) {
    try {
      new TextDecoder("utf-8", { fatal: true }).decode(bytes.subarray(0, length));
      return length;
    } catch {
      // UTF-8 字符最多四字节，只需回退片段末尾的三个字节。
    }
  }
  return 0;
}
