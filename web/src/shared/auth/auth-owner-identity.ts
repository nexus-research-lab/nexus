// INPUT: 服务端认证事实中的登录状态、owner ID、用户名与是否要求认证。
// OUTPUT: 稳定的本地 owner scope 投影及既有持久 marker 格式判定。
// POS: 无副作用的身份字符串合同；不读写存储、不推进 generation，也不清理运行态或授予权限。

import type { AuthStatus } from "@/lib/api/account/auth-api";

const LOCAL_SYSTEM_OWNER_SCOPE = "local-system";
const MAX_OWNER_IDENTITY_LENGTH = 512;

/** 仅构造客户端隔离标识；服务端认证和业务身份仍以原事实为准。 */
export function resolveAuthOwnerScope(
  status: Pick<AuthStatus, "authenticated" | "auth_required" | "user_id" | "username">,
): string | null {
  if (!status.authenticated) {
    return null;
  }
  const userId = normalizeOwnerIdentity(status.user_id);
  if (userId) {
    return `user-id:${userId}`;
  }
  const username = normalizeOwnerIdentity(status.username);
  if (username) {
    return `username:${username}`;
  }
  // Desktop Local 没有显式 Principal 时仍使用原有稳定隔离域。
  return status.auth_required ? null : LOCAL_SYSTEM_OWNER_SCOPE;
}

export function isValidPersistedAuthOwnerScope(value: string): boolean {
  return value === LOCAL_SYSTEM_OWNER_SCOPE
    || value.startsWith("user-id:")
    || value.startsWith("username:");
}

function normalizeOwnerIdentity(value: string | null | undefined): string {
  const normalized = value?.trim() ?? "";
  return normalized.length > 0 && normalized.length <= MAX_OWNER_IDENTITY_LENGTH
    ? normalized
    : "";
}
