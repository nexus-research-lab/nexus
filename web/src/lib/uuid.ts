"use client";

/**
 * 生成前端本地 UUID。
 *
 * 局域网通过 HTTP 访问时，randomUUID 可能不可用；
 * 此处提供兼容降级，避免创建会话和消息时直接抛错。
 */

function fallbackUuidFromRandomValues(): string {
  const cryptoObject = globalThis.crypto;
  if (!cryptoObject?.getRandomValues) {
    return `fallback-${Date.now()}-${Math.random().toString(16).slice(2, 10)}`;
  }

  const bytes = new Uint8Array(16);
  cryptoObject.getRandomValues(bytes);

  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;

  const hex = Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join("");
  return [
    hex.slice(0, 8),
    hex.slice(8, 12),
    hex.slice(12, 16),
    hex.slice(16, 20),
    hex.slice(20, 32),
  ].join("-");
}

export function generateUuid(): string {
  const cryptoObject = globalThis.crypto;
  if (cryptoObject?.randomUUID) {
    return cryptoObject.randomUUID();
  }

  return fallbackUuidFromRandomValues();
}

export function getBrowserClientId(): string {
  const storage_key = 'nexus.browser_client_id';
  const storage = globalThis.sessionStorage;
  if (!storage) {
    return generateUuid();
  }

  try {
    const existing_client_id = storage.getItem(storage_key)?.trim();
    if (existing_client_id) {
      return existing_client_id;
    }

    const next_client_id = generateUuid();
    storage.setItem(storage_key, next_client_id);
    return next_client_id;
  } catch {
    return generateUuid();
  }
}
