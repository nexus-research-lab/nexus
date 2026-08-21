/** 私聊 Session、可取消消息分页、大内容 detail 和轮次索引的 HTTP 边界。 */

import type {
  ApiConversation,
  Conversation,
} from "@/types/conversation/conversation";
import type {
  ApiAgentSession as ApiAgentSessionRecord,
  AgentSession as AgentSessionRecord,
} from "@/types/agent/agent";
import type {
  ApiConversationMessagePage,
  ApiSessionRoundIndex,
  ConversationMessagePage,
  ConversationMessagesQuery,
  MessageDetailResponse,
  SessionRoundIndexItem,
} from "@/types/conversation/history";
import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { applyDesktopRequestHeaders } from "@/config/desktop-runtime";
import { requestApi } from "@/lib/api/core/http";
import { assertStructuredSessionKey } from "@/lib/conversation/session-key";

import {
  buildConversationMessagesQuerySuffix,
  normalizeConversationMessagePage,
} from "./message-page-model";
import {
  transformApiAgentSession,
  transformApiConversation,
  transformApiSessionRoundIndex,
} from "./session-api-model";

const AGENT_API_BASE_URL = getAgentApiBaseUrl();

export interface SessionRuntimeSettings {
  provider: string;
  model: string;
  permission_mode: string;
  connector_ids: string[] | null;
}

export type SessionEchoMode = "inherit" | "enabled" | "disabled";

export interface SessionEchoOverride {
  mode: SessionEchoMode;
  enabled: boolean;
}

export interface SessionLocalDirectories {
  directories: string[];
}

export const getConversations = async (): Promise<Conversation[]> => {
  const result = await requestApi<ApiConversation[]>(
    `${AGENT_API_BASE_URL}/sessions`,
    {
      method: "GET",
    },
  );
  return result.map(transformApiConversation);
};

export const getAllSessionsApi = async (): Promise<AgentSessionRecord[]> => {
  const result = await requestApi<ApiAgentSessionRecord[]>(
    `${AGENT_API_BASE_URL}/sessions`,
    { method: "GET" },
  );
  return result.map(transformApiAgentSession);
};

export const getAgentSessionsApi = async (
  agentId: string,
): Promise<AgentSessionRecord[]> => {
  const result = await requestApi<ApiAgentSessionRecord[]>(
    `${AGENT_API_BASE_URL}/agents/${encodeURIComponent(agentId)}/sessions`,
    {
      method: "GET",
    },
  );
  return result.map(transformApiAgentSession);
};

export async function deleteSessionApi(sessionKey: string): Promise<void> {
  const normalizedSessionKey = assertStructuredSessionKey(sessionKey);
  await requestApi<{ success: boolean }>(
    `${AGENT_API_BASE_URL}/sessions/${encodeURIComponent(normalizedSessionKey)}`,
    { method: "DELETE" },
  );
}

export async function getSessionMessagesApi(
  sessionKey: string,
  options: ConversationMessagesQuery = {},
  signal?: AbortSignal,
): Promise<ConversationMessagePage> {
  const normalizedSessionKey = assertStructuredSessionKey(sessionKey);
  const querySuffix = buildConversationMessagesQuerySuffix(options, [
    ["session_key", normalizedSessionKey],
  ]);
  const result = await requestApi<ApiConversationMessagePage>(
    `${AGENT_API_BASE_URL}/sessions/messages${querySuffix}`,
    {
      method: "GET",
      signal,
    },
  );
  return normalizeConversationMessagePage(result);
}

export async function getSessionMessageDetailApi(
  sessionKey: string,
  detailRef: string,
  signal?: AbortSignal,
): Promise<MessageDetailResponse> {
  return requestApi<MessageDetailResponse>(
    buildSessionMessageDetailUrl(sessionKey, detailRef),
    { method: "GET", signal },
  );
}

export async function getSessionMessageImageDetailApi(
  sessionKey: string,
  detailRef: string,
  signal?: AbortSignal,
): Promise<Blob> {
  const url = buildSessionMessageDetailUrl(sessionKey, detailRef);
  const headers = new Headers();
  applyDesktopRequestHeaders(url, headers);
  const response = await fetch(url, {
    credentials: "include",
    headers,
    method: "GET",
    signal,
  });
  if (!response.ok) {
    throw new Error(`读取图片详情失败：HTTP ${response.status}`);
  }
  return response.blob();
}

function buildSessionMessageDetailUrl(
  sessionKey: string,
  detailRef: string,
): string {
  const normalizedSessionKey = assertStructuredSessionKey(sessionKey);
  const params = new URLSearchParams({
    detail_ref: detailRef.trim(),
    session_key: normalizedSessionKey,
  });
  return `${AGENT_API_BASE_URL}/sessions/message-detail?${params.toString()}`;
}

export async function getSessionRoundIndexApi(
  sessionKey: string,
  signal?: AbortSignal,
): Promise<SessionRoundIndexItem[]> {
  const normalizedSessionKey = assertStructuredSessionKey(sessionKey);
  const params = new URLSearchParams();
  params.set("session_key", normalizedSessionKey);
  params.set("defer_index", "true");
  for (;;) {
    const result = await requestApi<ApiSessionRoundIndex>(
      `${AGENT_API_BASE_URL}/sessions/rounds?${params.toString()}`,
      {
        method: "GET",
        signal,
      },
    );
    if (!result.indexing) {
      return transformApiSessionRoundIndex(result);
    }
    await waitForSessionRoundIndex(result.retry_after_ms ?? 0, signal);
  }
}

function waitForSessionRoundIndex(
  retryAfterMs: number,
  signal?: AbortSignal,
): Promise<void> {
  if (signal?.aborted) {
    return Promise.reject(new DOMException("Aborted", "AbortError"));
  }
  const delay = Math.min(Math.max(retryAfterMs, 100), 5_000);
  return new Promise((resolve, reject) => {
    const timeout = globalThis.setTimeout(() => {
      signal?.removeEventListener("abort", handleAbort);
      resolve();
    }, delay);
    const handleAbort = (): void => {
      globalThis.clearTimeout(timeout);
      reject(new DOMException("Aborted", "AbortError"));
    };
    signal?.addEventListener("abort", handleAbort, {once: true});
  });
}

export async function getSessionRuntimeSettingsApi(
  sessionKey: string,
): Promise<SessionRuntimeSettings> {
  const normalizedSessionKey = assertStructuredSessionKey(sessionKey);
  return requestApi<SessionRuntimeSettings>(
    `${AGENT_API_BASE_URL}/sessions/${encodeURIComponent(normalizedSessionKey)}/runtime-settings`,
    {
      method: "GET",
    },
  );
}

export async function updateSessionRuntimeSettingsApi(
  sessionKey: string,
  settings: SessionRuntimeSettings,
): Promise<SessionRuntimeSettings> {
  const normalizedSessionKey = assertStructuredSessionKey(sessionKey);
  return requestApi<SessionRuntimeSettings>(
    `${AGENT_API_BASE_URL}/sessions/${encodeURIComponent(normalizedSessionKey)}/runtime-settings`,
    {
      method: "PUT",
      body: JSON.stringify(settings),
    },
  );
}

export async function getSessionEchoApi(
  sessionKey: string,
): Promise<SessionEchoOverride> {
  const normalizedSessionKey = assertStructuredSessionKey(sessionKey);
  return requestApi<SessionEchoOverride>(
    `${AGENT_API_BASE_URL}/sessions/${encodeURIComponent(normalizedSessionKey)}/echo`,
    { method: "GET" },
  );
}

export async function updateSessionEchoApi(
  sessionKey: string,
  mode: SessionEchoMode,
): Promise<SessionEchoOverride> {
  const normalizedSessionKey = assertStructuredSessionKey(sessionKey);
  return requestApi<SessionEchoOverride>(
    `${AGENT_API_BASE_URL}/sessions/${encodeURIComponent(normalizedSessionKey)}/echo`,
    {
      method: "PUT",
      body: JSON.stringify({ mode }),
    },
  );
}

export async function getSessionLocalDirectoriesApi(
  sessionKey: string,
): Promise<SessionLocalDirectories> {
  const normalizedSessionKey = assertStructuredSessionKey(sessionKey);
  return requestApi<SessionLocalDirectories>(
    `${AGENT_API_BASE_URL}/sessions/${encodeURIComponent(normalizedSessionKey)}/local-directories`,
    { method: "GET" },
  );
}

export async function updateSessionLocalDirectoriesApi(
  sessionKey: string,
  directories: string[],
): Promise<SessionLocalDirectories> {
  const normalizedSessionKey = assertStructuredSessionKey(sessionKey);
  return requestApi<SessionLocalDirectories>(
    `${AGENT_API_BASE_URL}/sessions/${encodeURIComponent(normalizedSessionKey)}/local-directories`,
    {
      method: "PUT",
      body: JSON.stringify({ directories }),
    },
  );
}
