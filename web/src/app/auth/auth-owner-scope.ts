/**
 * INPUT: 服务端权威 AuthStatus、跨标签页 owner marker 与认证生命周期。
 * OUTPUT: 先推进事件 generation，再跨 owner/登出同步清空的客户端状态。
 * POS: AuthProvider 使用的用户作用域栅栏；不改变服务端身份、资源 ID 或请求语义。
 */

import type { AuthStatus } from "@/lib/api/account/auth-api";
import { resetSharedWebSocketsOwnerScope } from "@/lib/websocket";
import { resetRuntimeOptionsForOwnerChange } from "@/config/runtime-options";
import { resetRoomActivityOwnerScope } from "@/features/home/room-activity-resource";
import {
  resetVolatileConversationOwnerScope,
  setVolatileConversationOwnerScope,
} from "@/hooks/agent/runtime/snapshot/conversation-volatile-storage";
import { resetAgentOwnerScope, setAgentOwnerScope } from "@/store/agent";
import { resetSidebarOwnerScope } from "@/store/sidebar";
import { resetWorkspaceFilesOwnerScope } from "@/store/workspace-files";
import { resetWorkspaceLiveOwnerScope } from "@/store/workspace-live";
import {
  advanceAuthOwnerScopeGeneration,
  publishAuthOwnerScopeGeneration,
} from "@/shared/auth/auth-owner-generation";
import { resetComposerDraftOwnerScope } from "@/features/conversation/shared/composer/composer-draft-store";
import { resetComposerHistoryOwnerScope } from "@/features/conversation/shared/composer/composer-history-store";
import { resetHomeDirectoryOwnerScope } from "@/features/home/home-directory-resource";
import { resetConversationOwnerScope } from "@/store/conversation";
import { resetRoomNavigationOwnerScope } from "@/store/room-navigation";

export const AUTH_OWNER_SCOPE_STORAGE_KEY = "nexus-auth-owner-scope";

const LOCAL_SYSTEM_OWNER_SCOPE = "local-system";
const MAX_OWNER_IDENTITY_LENGTH = 512;

let activeOwnerScope: string | null | undefined;

/** 从服务端身份构造只用于本地隔离的稳定 scope，不把它用作业务身份。 */
export function resolveAuthOwnerScope(status: AuthStatus): string | null {
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

  // 关闭认证的本地单用户模式可能没有显式 Principal，仍需一个稳定隔离域。
  return status.auth_required ? null : LOCAL_SYSTEM_OWNER_SCOPE;
}

/**
 * 在任何 owner 数据进入 React 状态前推进本地身份栅栏。
 * 返回 true 表示发生了首次安全迁移、登出或 owner 切换。
 */
export function applyAuthOwnerScope(status: AuthStatus): boolean {
  const nextOwnerScope = resolveAuthOwnerScope(status);
  const previousOwnerScope = activeOwnerScope === undefined
    ? readPersistedOwnerScope()
    : activeOwnerScope;
  const scopeChanged = previousOwnerScope === undefined
    || previousOwnerScope !== nextOwnerScope;

  if (scopeChanged) {
    resetOwnerScopedClientState(nextOwnerScope !== null, nextOwnerScope);
  } else {
    // 同 owner 刷新也要初始化进程内 namespace，才能安全恢复本标签页快照。
    setVolatileConversationOwnerScope(nextOwnerScope);
    setAgentOwnerScope(nextOwnerScope);
  }
  activeOwnerScope = nextOwnerScope;
  persistOwnerScope(nextOwnerScope);
  return scopeChanged;
}

/** 跨标签页认证变化先清空当前内存，不改写另一标签页刚提交的 marker。 */
export function invalidateLocalAuthOwnerScope(): void {
  activeOwnerScope = null;
  resetOwnerScopedClientState(false, null);
}

export function isAuthOwnerScopeStorageEvent(event: StorageEvent): boolean {
  return event.key === AUTH_OWNER_SCOPE_STORAGE_KEY;
}

function resetOwnerScopedClientState(
  reloadHomeDirectory: boolean,
  nextOwnerScope: string | null,
): void {
  // 必须先废弃旧订阅，再逐项清空；React 卸载与共享 WebSocket cleanup 都晚于这里。
  advanceAuthOwnerScopeGeneration();
  resetSharedWebSocketsOwnerScope();
  resetVolatileConversationOwnerScope(nextOwnerScope);
  resetRuntimeOptionsForOwnerChange();
  resetHomeDirectoryOwnerScope(reloadHomeDirectory);
  resetRoomActivityOwnerScope();
  resetAgentOwnerScope(nextOwnerScope);
  resetConversationOwnerScope();
  resetRoomNavigationOwnerScope();
  resetSidebarOwnerScope();
  resetWorkspaceFilesOwnerScope();
  resetWorkspaceLiveOwnerScope();
  resetComposerDraftOwnerScope();
  resetComposerHistoryOwnerScope();
  // 所有旧状态与连接都已失效后，再让仍挂载的 Hook 为新 owner 建立资源。
  publishAuthOwnerScopeGeneration();
}

function normalizeOwnerIdentity(value: string | null | undefined): string {
  const normalized = value?.trim() ?? "";
  return normalized.length > 0 && normalized.length <= MAX_OWNER_IDENTITY_LENGTH
    ? normalized
    : "";
}

function readPersistedOwnerScope(): string | null | undefined {
  if (typeof window === "undefined") {
    return undefined;
  }
  try {
    const value = window.localStorage.getItem(AUTH_OWNER_SCOPE_STORAGE_KEY);
    if (value === null) {
      return undefined;
    }
    return isValidPersistedOwnerScope(value) ? value : undefined;
  } catch {
    // 无法读取 marker 时按未迁移状态处理，宁可清空也不认领未知 owner 数据。
    return undefined;
  }
}

function persistOwnerScope(ownerScope: string | null): void {
  if (typeof window === "undefined") {
    return;
  }
  try {
    if (ownerScope === null) {
      window.localStorage.removeItem(AUTH_OWNER_SCOPE_STORAGE_KEY);
      return;
    }
    window.localStorage.setItem(AUTH_OWNER_SCOPE_STORAGE_KEY, ownerScope);
  } catch {
    // marker 无法持久化时，下次启动会再次安全清空，不影响当前认证功能。
  }
}

function isValidPersistedOwnerScope(value: string): boolean {
  return value === LOCAL_SYSTEM_OWNER_SCOPE
    || value.startsWith("user-id:")
    || value.startsWith("username:");
}
