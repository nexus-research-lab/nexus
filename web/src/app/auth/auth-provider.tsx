/**
 * INPUT: 服务端认证状态、登录/登出命令、401 与跨标签页 owner 变化。
 * OUTPUT: 在 owner 客户端状态完成隔离后发布的 Auth Context 与运行时配置刷新。
 * POS: 应用认证装配层；业务缓存身份只由 auth-owner-scope 统一推进。
 */

"use client";

import {
  ReactNode,
  startTransition,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";

import { hydrateRuntimeOptions } from "@/app/runtime-options-resource";
import { AuthStatus, getAuthStatus, loginApi, logoutApi } from "@/lib/api/account/auth-api";
import { AUTH_REQUIRED_EVENT } from "@/lib/api/core/http-auth";
import { getErrorMessage } from "@/lib/error-message";
import { AUTH_CONTEXT } from "@/shared/auth/auth-context";

import {
  applyAuthOwnerScope,
  invalidateLocalAuthOwnerScope,
  isAuthOwnerScopeStorageEvent,
} from "./auth-owner-scope";
import { runAuthStatusBootstrap } from "./auth-status-bootstrap";
import { isAuthOwnerScopeSupersededError } from "@/shared/auth/auth-owner-generation";

const DEFAULT_UNAUTHORIZED_STATUS: AuthStatus = {
  auth_required: true,
  password_login_enabled: true,
  authenticated: false,
  username: null,
  user_id: null,
  display_name: null,
  role: null,
  avatar: null,
  auth_method: null,
};

const RUNTIME_OPTIONS_SCOPE_REFRESH_ERROR =
  "登录状态已更新，但当前账号的运行配置没有加载成功。已有账户数据没有被修改；刷新页面后重试。";

export function AuthProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<AuthStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [isBootstrapped, setIsBootstrapped] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const hidePreviousOwnerSurface = useCallback((scopeChanged: boolean) => {
    if (!scopeChanged) {
      return;
    }
    // generation 已推进但新配置尚未完成时，必须同步撤下旧页面；否则仍挂载的
    // 表单可能拿新 generation 提交旧 owner 的 draft/ID。
    setLoading(true);
    setError(null);
    setIsBootstrapped(false);
    setStatus(null);
  }, []);

  const refreshStatus = useCallback(async (): Promise<AuthStatus> => {
    setLoading(true);
    let superseded = false;
    try {
      return await runAuthStatusBootstrap(
        getAuthStatus,
        async (nextStatus) => {
          const scopeChanged = applyAuthOwnerScope(nextStatus);
          hidePreviousOwnerSurface(scopeChanged);
          const runtimeOptionsError = await refreshRuntimeOptionsForOwnerChange(
            scopeChanged,
          );
          startTransition(() => {
            setStatus(nextStatus);
            setError(runtimeOptionsError);
            setIsBootstrapped(true);
          });
          return nextStatus;
        },
      );
    } catch (err) {
      if (isAuthOwnerScopeSupersededError(err)) {
        superseded = true;
        throw err;
      }
      const message = getErrorMessage(err, "登录状态暂时无法加载");
      startTransition(() => {
        setError(message);
        setIsBootstrapped(true);
      });
      throw err;
    } finally {
      if (!superseded) {
        setLoading(false);
      }
    }
  }, [hidePreviousOwnerSurface]);

  useEffect(() => {
    void refreshStatus().catch((err) => {
      if (isAuthOwnerScopeSupersededError(err)) {
        return;
      }
      console.warn("[AuthProvider] Auth bootstrap failed:", err instanceof Error ? err.message : err);
    });
  }, [refreshStatus]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return undefined;
    }

    const handleAuthRequired = () => {
      applyAuthOwnerScope(DEFAULT_UNAUTHORIZED_STATUS);
      setLoading(false);
      setError(null);
      setIsBootstrapped(true);
      setStatus((currentStatus) => {
        if (!currentStatus) {
          return DEFAULT_UNAUTHORIZED_STATUS;
        }
        return {
          ...currentStatus,
          authenticated: false,
          username: null,
          user_id: null,
          display_name: null,
          role: null,
          avatar: null,
          auth_method: null,
        };
      });
    };

    const handleOwnerScopeStorageChange = (event: StorageEvent) => {
      if (!isAuthOwnerScopeStorageEvent(event)) {
        return;
      }
      // Cookie 与 localStorage 都跨标签页共享；先隐藏旧 owner，再读取服务端权威身份。
      invalidateLocalAuthOwnerScope();
      setError(null);
      setIsBootstrapped(false);
      setStatus(null);
      void refreshStatus().catch((err) => {
        if (isAuthOwnerScopeSupersededError(err)) {
          return;
        }
        console.warn(
          "[AuthProvider] Auth scope refresh failed:",
          err instanceof Error ? err.message : err,
        );
      });
    };

    window.addEventListener(AUTH_REQUIRED_EVENT, handleAuthRequired);
    window.addEventListener("storage", handleOwnerScopeStorageChange);
    return () => {
      window.removeEventListener(AUTH_REQUIRED_EVENT, handleAuthRequired);
      window.removeEventListener("storage", handleOwnerScopeStorageChange);
    };
  }, [refreshStatus]);

  const login = useCallback(async (username: string, password: string): Promise<AuthStatus> => {
    const nextStatus = await loginApi({ username, password });
    const scopeChanged = applyAuthOwnerScope(nextStatus);
    hidePreviousOwnerSurface(scopeChanged);
    const runtimeOptionsError = await refreshRuntimeOptionsForOwnerChange(
      scopeChanged,
    );
    startTransition(() => {
      setLoading(false);
      setStatus(nextStatus);
      setError(runtimeOptionsError);
      setIsBootstrapped(true);
    });
    return nextStatus;
  }, [hidePreviousOwnerSurface]);

  const logout = useCallback(async (): Promise<AuthStatus> => {
    const nextStatus = await logoutApi();
    const scopeChanged = applyAuthOwnerScope(nextStatus);
    hidePreviousOwnerSurface(scopeChanged);
    const runtimeOptionsError = await refreshRuntimeOptionsForOwnerChange(
      scopeChanged,
    );
    startTransition(() => {
      setLoading(false);
      setStatus(nextStatus);
      setError(runtimeOptionsError);
      setIsBootstrapped(true);
    });
    return nextStatus;
  }, [hidePreviousOwnerSurface]);

  const contextValue = useMemo(() => ({
    status,
    loading,
    isBootstrapped,
    error,
    refreshStatus,
    login,
    logout,
  }), [error, isBootstrapped, loading, login, logout, refreshStatus, status]);

  return (
    <AUTH_CONTEXT.Provider
      value={contextValue}
    >
      {children}
    </AUTH_CONTEXT.Provider>
  );
}

async function refreshRuntimeOptionsForOwnerChange(
  scopeChanged: boolean,
): Promise<string | null> {
  if (!scopeChanged) {
    return null;
  }
  try {
    await hydrateRuntimeOptions();
    return null;
  } catch (error) {
    if (isAuthOwnerScopeSupersededError(error)) {
      throw error;
    }
    console.warn(
      "[AuthProvider] Runtime options refresh failed after owner change:",
      error instanceof Error ? error.message : error,
    );
    return RUNTIME_OPTIONS_SCOPE_REFRESH_ERROR;
  }
}
