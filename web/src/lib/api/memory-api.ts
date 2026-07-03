import { getAgentApiBaseUrl } from "@/config/options";
import { requestApi } from "@/lib/api/http";
import type {
  MemoryCleanupResult,
  MemoryInjection,
  MemoryItem,
  MemoryStats,
  MemoryWriteInput,
} from "@/types/memory/memory";

const AGENT_API_BASE_URL = getAgentApiBaseUrl();

interface MemoryItemsResponse {
  items: MemoryItem[];
}

function agentMemoryBaseUrl(agentId: string): string {
  return `${AGENT_API_BASE_URL}/agents/${encodeURIComponent(agentId)}/memory`;
}

function userMemoryBaseUrl(): string {
  return `${AGENT_API_BASE_URL}/memory`;
}

function memoryItemsQuery(params: { limit?: number; status?: string; scope?: string } = {}): string {
  const query = new URLSearchParams();
  if (params.limit) {
    query.set("limit", String(params.limit));
  }
  if (params.status) {
    query.set("status", params.status);
  }
  if (params.scope) {
    query.set("scope", params.scope);
  }
  return query.toString() ? `?${query.toString()}` : "";
}

export async function listMemoryItemsApi(
  agentId: string,
  params: { limit?: number; status?: string; scope?: string } = {},
): Promise<MemoryItem[]> {
  const suffix = memoryItemsQuery(params);
  const result = await requestApi<MemoryItemsResponse>(
    `${agentMemoryBaseUrl(agentId)}/items${suffix}`,
    { method: "GET" },
  );
  return result.items;
}

export async function listUserMemoryItemsApi(
  params: { limit?: number; status?: string; scope?: string } = {},
): Promise<MemoryItem[]> {
  const suffix = memoryItemsQuery(params);
  const result = await requestApi<MemoryItemsResponse>(
    `${userMemoryBaseUrl()}/items${suffix}`,
    { method: "GET" },
  );
  return result.items;
}

export async function searchMemoryItemsApi(
  agentId: string,
  queryText: string,
  limit = 8,
): Promise<MemoryItem[]> {
  const query = new URLSearchParams({ q: queryText, limit: String(limit) });
  const result = await requestApi<MemoryItemsResponse>(
    `${agentMemoryBaseUrl(agentId)}/search?${query.toString()}`,
    { method: "GET" },
  );
  return result.items;
}

export async function searchUserMemoryItemsApi(
  queryText: string,
  limit = 8,
): Promise<MemoryItem[]> {
  const query = new URLSearchParams({ q: queryText, limit: String(limit) });
  const result = await requestApi<MemoryItemsResponse>(
    `${userMemoryBaseUrl()}/search?${query.toString()}`,
    { method: "GET" },
  );
  return result.items;
}

export async function addUserMemoryItemApi(input: MemoryWriteInput): Promise<MemoryItem> {
  return requestApi<MemoryItem>(`${userMemoryBaseUrl()}/items`, {
    method: "POST",
    body: { ...input },
  });
}

export async function updateUserMemoryItemApi(
  entryId: string,
  input: MemoryWriteInput,
): Promise<MemoryItem> {
  return requestApi<MemoryItem>(
    `${userMemoryBaseUrl()}/items/${encodeURIComponent(entryId)}`,
    {
      method: "PATCH",
      body: { ...input },
    },
  );
}

export async function deleteMemoryItemApi(
  agentId: string,
  entryId: string,
): Promise<{ deleted: boolean }> {
  return requestApi<{ deleted: boolean }>(
    `${agentMemoryBaseUrl(agentId)}/items/${encodeURIComponent(entryId)}`,
    { method: "DELETE" },
  );
}

export async function deleteUserMemoryItemApi(
  entryId: string,
): Promise<{ deleted: boolean }> {
  return requestApi<{ deleted: boolean }>(
    `${userMemoryBaseUrl()}/items/${encodeURIComponent(entryId)}`,
    { method: "DELETE" },
  );
}

export async function promoteUserMemoryItemApi(
  entryId: string,
  target = "memory",
): Promise<{ path: string; content: string }> {
  return requestApi<{ path: string; content: string }>(
    `${userMemoryBaseUrl()}/items/${encodeURIComponent(entryId)}/promote`,
    {
      method: "POST",
      body: { target },
    },
  );
}

export async function ignoreUserMemoryItemApi(
  entryId: string,
  note = "",
): Promise<MemoryItem> {
  return requestApi<MemoryItem>(
    `${userMemoryBaseUrl()}/items/${encodeURIComponent(entryId)}/ignore`,
    {
      method: "POST",
      body: { note },
    },
  );
}

export async function getMemoryStatsApi(agentId: string): Promise<MemoryStats> {
  return requestApi<MemoryStats>(`${agentMemoryBaseUrl(agentId)}/stats`, {
    method: "GET",
  });
}

export async function getUserMemoryStatsApi(): Promise<MemoryStats> {
  return requestApi<MemoryStats>(`${userMemoryBaseUrl()}/stats`, {
    method: "GET",
  });
}

export async function cleanupMemoryApi(agentId: string): Promise<MemoryCleanupResult> {
  return requestApi<MemoryCleanupResult>(`${agentMemoryBaseUrl(agentId)}/cleanup`, {
    method: "POST",
    body: {},
  });
}

export async function cleanupUserMemoryApi(): Promise<MemoryCleanupResult> {
  return requestApi<MemoryCleanupResult>(`${userMemoryBaseUrl()}/cleanup`, {
    method: "POST",
    body: {},
  });
}
