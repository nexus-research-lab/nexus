/**
 * INPUT: 当前认证 owner scope 与一个非敏感 Agent 创建 request ID。
 * OUTPUT: 可选的跨重载创建意图。
 * POS: Agent 创建 receipt 的浏览器辅助；存储不可用不阻止创建。
 */

const STORAGE_VERSION = 1;
const STORAGE_PREFIX = "nexus:agent-creation-journal:v1:";

interface StoredAgentCreationIntent {
  requestId: string;
  version: typeof STORAGE_VERSION;
}

function key(ownerScope: string): string {
  return `${STORAGE_PREFIX}${encodeURIComponent(ownerScope)}`;
}

function storage(): Storage | null {
  try {
    return typeof window === "undefined" ? null : window.localStorage;
  } catch {
    return null;
  }
}

export function readAgentCreationRequestId(ownerScope: string | null): string | null {
  const target = storage();
  if (!ownerScope || !target) {
    return null;
  }
  try {
    const value = JSON.parse(
      target.getItem(key(ownerScope)) ?? "null",
    ) as Partial<StoredAgentCreationIntent> | null;
    return value?.version === STORAGE_VERSION
      && typeof value.requestId === "string"
      && value.requestId.trim()
      ? value.requestId.trim()
      : null;
  } catch {
    return null;
  }
}

export function saveAgentCreationRequestId(
  ownerScope: string | null,
  requestId: string,
): void {
  const target = storage();
  requestId = requestId.trim();
  if (!ownerScope || !target || !requestId) {
    return;
  }
  try {
    target.setItem(key(ownerScope), JSON.stringify({
      requestId,
      version: STORAGE_VERSION,
    } satisfies StoredAgentCreationIntent));
  } catch {
    // 当前请求仍携带服务端 receipt identity；本地存储不是提交门槛。
  }
}

export function clearAgentCreationRequestId(ownerScope: string | null): void {
  const target = storage();
  if (!ownerScope || !target) {
    return;
  }
  try {
    target.removeItem(key(ownerScope));
  } catch {
    // 清理失败不改变服务端 receipt。
  }
}

export function createAgentCreationRequestId(): string | null {
  try {
    if (typeof crypto === "undefined") {
      return null;
    }
    if (typeof crypto.randomUUID === "function") {
      return `web-create:${crypto.randomUUID()}`;
    }
    const bytes = new Uint8Array(16);
    crypto.getRandomValues(bytes);
    const token = Array.from(bytes, (value) => value.toString(16).padStart(2, "0")).join("");
    return `web-create:${token}`;
  } catch {
    return null;
  }
}
