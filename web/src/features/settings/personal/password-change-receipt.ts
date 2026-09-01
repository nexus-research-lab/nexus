/**
 * INPUT: 当前 owner 的 user_id 与 exact 密码修改 request_id。
 * OUTPUT: 不含密码的未对账回执指针，用于刷新后继续核对。
 * POS: Personal 密码 mutation 的窄持久边界；不保存凭据、草稿或结果猜测。
 */

const PASSWORD_CHANGE_RECEIPT_PREFIX = "nexus:password-change-receipt:";
const PASSWORD_CHANGE_REQUEST_ID_PATTERN = /^[A-Za-z0-9._:-]{8,128}$/;

function getStorage(): Storage | null {
  try {
    return typeof window === "undefined" ? null : window.localStorage;
  } catch {
    return null;
  }
}

function storageKey(userID: string): string {
  return `${PASSWORD_CHANGE_RECEIPT_PREFIX}${userID.trim()}`;
}

export function createPasswordChangeRequestID(): string {
  return `web-password:${globalThis.crypto.randomUUID()}`;
}

export function readPendingPasswordChangeRequest(userID: string): string | null {
  const storage = getStorage();
  if (!storage || !userID.trim()) {
    return null;
  }
  try {
    const value = storage.getItem(storageKey(userID))?.trim() ?? "";
    return PASSWORD_CHANGE_REQUEST_ID_PATTERN.test(value) ? value : null;
  } catch {
    return null;
  }
}

export function rememberPendingPasswordChangeRequest(
  userID: string,
  requestID: string,
): void {
  const storage = getStorage();
  if (
    !storage
    || !userID.trim()
    || !PASSWORD_CHANGE_REQUEST_ID_PATTERN.test(requestID.trim())
  ) {
    return;
  }
  try {
    storage.setItem(storageKey(userID), requestID.trim());
  } catch {
    // 持久化不可用时，当前页面的内存锁仍然保持安全。
  }
}

export function forgetPendingPasswordChangeRequest(
  userID: string,
  requestID: string,
): void {
  const storage = getStorage();
  if (!storage || !userID.trim() || !requestID.trim()) {
    return;
  }
  try {
    const key = storageKey(userID);
    if (storage.getItem(key)?.trim() === requestID.trim()) {
      storage.removeItem(key);
    }
  } catch {
    // 不把清理失败扩大为密码修改失败。
  }
}
