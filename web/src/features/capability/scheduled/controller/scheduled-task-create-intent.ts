// INPUT: 当前 owner scope 与 Scheduled 创建 request ID。
// OUTPUT: 可跨重载恢复的非敏感创建意图，以及跨标签页变更通知。
// POS: 创建回执恢复辅助；服务端 receipt 才是提交结果真相，存储不可用不阻止创建。

const STORAGE_VERSION = 3;
const STORAGE_PREFIX = "nexus:scheduled-mutation-journal:v3:";

interface StoredCreateIntent {
  requestId: string;
  updatedAt: number;
  version: typeof STORAGE_VERSION;
}

function storage(): Storage | null {
  try {
    return typeof window === "undefined" ? null : window.localStorage;
  } catch {
    return null;
  }
}

function createPrefix(scopeKey: string): string {
  return `${STORAGE_PREFIX}${encodeURIComponent(scopeKey)}:create:`;
}

function createKey(scopeKey: string, requestId: string): string {
  return `${createPrefix(scopeKey)}${encodeURIComponent(requestId)}`;
}

function storageKeys(target: Storage, prefix: string): string[] {
  const keys: string[] = [];
  for (let index = 0; index < target.length; index += 1) {
    const key = target.key(index);
    if (key?.startsWith(prefix)) {
      keys.push(key);
    }
  }
  return keys;
}

function parseCreateIntent(target: Storage, key: string): StoredCreateIntent | null {
  try {
    const value = JSON.parse(target.getItem(key) ?? "null") as Partial<StoredCreateIntent>;
    return value?.version === STORAGE_VERSION
      && typeof value.requestId === "string"
      && value.requestId.trim() !== ""
      && typeof value.updatedAt === "number"
      ? value as StoredCreateIntent
      : null;
  } catch {
    return null;
  }
}

export function loadScheduledTaskCreateRequestIds(
  scopeKey: string | null,
): string[] {
  const target = storage();
  if (!scopeKey || !target) {
    return [];
  }
  return storageKeys(target, createPrefix(scopeKey))
    .flatMap((key) => {
      const intent = parseCreateIntent(target, key);
      return intent ? [intent] : [];
    })
    .sort((left, right) => (
      left.updatedAt - right.updatedAt || left.requestId.localeCompare(right.requestId)
    ))
    .map((intent) => intent.requestId);
}

export function loadScheduledTaskCreateRequestId(
  scopeKey: string | null,
): string | null {
  return loadScheduledTaskCreateRequestIds(scopeKey)[0] ?? null;
}

export function saveScheduledTaskCreateRequestId(
  scopeKey: string | null,
  requestId: string,
): void {
  const target = storage();
  requestId = requestId.trim();
  if (!scopeKey || !target || !requestId) {
    return;
  }
  try {
    target.setItem(createKey(scopeKey, requestId), JSON.stringify({
      requestId,
      updatedAt: Date.now(),
      version: STORAGE_VERSION,
    } satisfies StoredCreateIntent));
  } catch {
    // 当前页面仍可用同一 request ID 查询服务端回执；存储不是提交门槛。
  }
}

export function clearScheduledTaskCreateRequestId(
  scopeKey: string | null,
  requestId: string,
): void {
  const target = storage();
  requestId = requestId.trim();
  if (!scopeKey || !target || !requestId) {
    return;
  }
  try {
    target.removeItem(createKey(scopeKey, requestId));
  } catch {
    // 清理失败不改变服务端回执。
  }
}

export function subscribeScheduledTaskCreateIntents(
  scopeKey: string | null,
  listener: () => void,
): () => void {
  if (!scopeKey || typeof window === "undefined") {
    return () => undefined;
  }
  const prefix = createPrefix(scopeKey);
  const handleStorage = (event: StorageEvent): void => {
    if (event.storageArea === window.localStorage && (
      event.key === null || event.key.startsWith(prefix)
    )) {
      listener();
    }
  };
  window.addEventListener("storage", handleStorage);
  return () => window.removeEventListener("storage", handleStorage);
}
