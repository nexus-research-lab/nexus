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

function agent_memory_base_url(agent_id: string): string {
  return `${AGENT_API_BASE_URL}/agents/${encodeURIComponent(agent_id)}/memory`;
}

export async function list_memory_items_api(
  agent_id: string,
  params: { limit?: number; status?: string; scope?: string } = {},
): Promise<MemoryItem[]> {
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
  const suffix = query.toString() ? `?${query.toString()}` : "";
  const result = await request_api<MemoryItemsResponse>(
    `${agent_memory_base_url(agent_id)}/items${suffix}`,
    { method: "GET" },
  );
  return result.items;
}

export async function search_memory_items_api(
  agent_id: string,
  query_text: string,
  limit = 8,
): Promise<MemoryItem[]> {
  const query = new URLSearchParams({ q: query_text, limit: String(limit) });
  const result = await request_api<MemoryItemsResponse>(
    `${agent_memory_base_url(agent_id)}/search?${query.toString()}`,
    { method: "GET" },
  );
  return result.items;
}

export async function recall_memory_api(
  agent_id: string,
  query_text: string,
  max_results = 5,
): Promise<MemoryInjection> {
  return request_api<MemoryInjection>(`${agent_memory_base_url(agent_id)}/recall`, {
    method: "POST",
    body: { query: query_text, max_results },
  });
}

export async function add_memory_item_api(
  agent_id: string,
  input: MemoryWriteInput,
): Promise<MemoryItem> {
  return request_api<MemoryItem>(`${agent_memory_base_url(agent_id)}/items`, {
    method: "POST",
    body: { ...input },
  });
}

export async function update_memory_item_api(
  agent_id: string,
  entry_id: string,
  input: MemoryWriteInput,
): Promise<MemoryItem> {
  return request_api<MemoryItem>(
    `${agent_memory_base_url(agent_id)}/items/${encodeURIComponent(entry_id)}`,
    {
      method: "PATCH",
      body: { ...input },
    },
  );
}

export async function delete_memory_item_api(
  agent_id: string,
  entry_id: string,
): Promise<{ deleted: boolean }> {
  return request_api<{ deleted: boolean }>(
    `${agent_memory_base_url(agent_id)}/items/${encodeURIComponent(entry_id)}`,
    { method: "DELETE" },
  );
}

export async function promote_memory_item_api(
  agent_id: string,
  entry_id: string,
  target = "memory",
): Promise<{ path: string; content: string }> {
  return request_api<{ path: string; content: string }>(
    `${agent_memory_base_url(agent_id)}/items/${encodeURIComponent(entry_id)}/promote`,
    {
      method: "POST",
      body: { target },
    },
  );
}

export async function ignore_memory_item_api(
  agent_id: string,
  entry_id: string,
  note = "",
): Promise<MemoryItem> {
  return request_api<MemoryItem>(
    `${agent_memory_base_url(agent_id)}/items/${encodeURIComponent(entry_id)}/ignore`,
    {
      method: "POST",
      body: { note },
    },
  );
}

export async function get_memory_stats_api(agent_id: string): Promise<MemoryStats> {
  return request_api<MemoryStats>(`${agent_memory_base_url(agent_id)}/stats`, {
    method: "GET",
  });
}

export async function cleanup_memory_api(agent_id: string): Promise<MemoryCleanupResult> {
  return request_api<MemoryCleanupResult>(`${agent_memory_base_url(agent_id)}/cleanup`, {
    method: "POST",
    body: {},
  });
}
