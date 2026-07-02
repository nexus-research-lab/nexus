import { get_agent_api_base_url } from "@/config/options";
import { request_api } from "@/lib/api/http";
import type {
  MemoryCleanupResult,
  MemoryInjection,
  MemoryItem,
  MemoryStats,
  MemoryWriteInput,
} from "@/types/memory/memory";

const AGENT_API_BASE_URL = get_agent_api_base_url();

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

export async function list_memory_items_api(
  agentId: string,
  params: { limit?: number; status?: string; scope?: string } = {},
): Promise<MemoryItem[]> {
  const suffix = memoryItemsQuery(params);
  const result = await request_api<MemoryItemsResponse>(
    `${agentMemoryBaseUrl(agentId)}/items${suffix}`,
    { method: "GET" },
  );
  return result.items;
}

export async function list_user_memory_items_api(
  params: { limit?: number; status?: string; scope?: string } = {},
): Promise<MemoryItem[]> {
  const suffix = memoryItemsQuery(params);
  const result = await request_api<MemoryItemsResponse>(
    `${userMemoryBaseUrl()}/items${suffix}`,
    { method: "GET" },
  );
  return result.items;
}

export async function search_memory_items_api(
  agentId: string,
  queryText: string,
  limit = 8,
): Promise<MemoryItem[]> {
  const query = new URLSearchParams({ q: queryText, limit: String(limit) });
  const result = await request_api<MemoryItemsResponse>(
    `${agentMemoryBaseUrl(agentId)}/search?${query.toString()}`,
    { method: "GET" },
  );
  return result.items;
}

export async function search_user_memory_items_api(
  queryText: string,
  limit = 8,
): Promise<MemoryItem[]> {
  const query = new URLSearchParams({ q: queryText, limit: String(limit) });
  const result = await request_api<MemoryItemsResponse>(
    `${userMemoryBaseUrl()}/search?${query.toString()}`,
    { method: "GET" },
  );
  return result.items;
}

export async function add_user_memory_item_api(input: MemoryWriteInput): Promise<MemoryItem> {
  return request_api<MemoryItem>(`${userMemoryBaseUrl()}/items`, {
    method: "POST",
    body: { ...input },
  });
}

export async function update_user_memory_item_api(
  entryId: string,
  input: MemoryWriteInput,
): Promise<MemoryItem> {
  return request_api<MemoryItem>(
    `${userMemoryBaseUrl()}/items/${encodeURIComponent(entryId)}`,
    {
      method: "PATCH",
      body: { ...input },
    },
  );
}

export async function delete_memory_item_api(
  agentId: string,
  entryId: string,
): Promise<{ deleted: boolean }> {
  return request_api<{ deleted: boolean }>(
    `${agentMemoryBaseUrl(agentId)}/items/${encodeURIComponent(entryId)}`,
    { method: "DELETE" },
  );
}

export async function delete_user_memory_item_api(
  entryId: string,
): Promise<{ deleted: boolean }> {
  return request_api<{ deleted: boolean }>(
    `${userMemoryBaseUrl()}/items/${encodeURIComponent(entryId)}`,
    { method: "DELETE" },
  );
}

export async function promote_user_memory_item_api(
  entryId: string,
  target = "memory",
): Promise<{ path: string; content: string }> {
  return request_api<{ path: string; content: string }>(
    `${userMemoryBaseUrl()}/items/${encodeURIComponent(entryId)}/promote`,
    {
      method: "POST",
      body: { target },
    },
  );
}

export async function ignore_user_memory_item_api(
  entryId: string,
  note = "",
): Promise<MemoryItem> {
  return request_api<MemoryItem>(
    `${userMemoryBaseUrl()}/items/${encodeURIComponent(entryId)}/ignore`,
    {
      method: "POST",
      body: { note },
    },
  );
}

export async function get_memory_stats_api(agentId: string): Promise<MemoryStats> {
  return request_api<MemoryStats>(`${agentMemoryBaseUrl(agentId)}/stats`, {
    method: "GET",
  });
}

export async function get_user_memory_stats_api(): Promise<MemoryStats> {
  return request_api<MemoryStats>(`${userMemoryBaseUrl()}/stats`, {
    method: "GET",
  });
}

export async function cleanup_memory_api(agentId: string): Promise<MemoryCleanupResult> {
  return request_api<MemoryCleanupResult>(`${agentMemoryBaseUrl(agentId)}/cleanup`, {
    method: "POST",
    body: {},
  });
}

export async function cleanup_user_memory_api(): Promise<MemoryCleanupResult> {
  return request_api<MemoryCleanupResult>(`${userMemoryBaseUrl()}/cleanup`, {
    method: "POST",
    body: {},
  });
}
