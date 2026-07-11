/**
 * =====================================================
 * @File   : auth-provider.tsx
 * @Date   : 2026-04-07 18:24
 * @Author : leemysw
 * 2026-04-07 18:24   Create
 * =====================================================
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

import { hydrateRuntimeOptions } from "@/config/options";
import { AuthStatus, getAuthStatus, loginApi, logoutApi } from "@/lib/api/account/auth-api";
import { AUTH_REQUIRED_EVENT } from "@/lib/api/core/http";
import { AUTH_CONTEXT } from "@/shared/auth/auth-context";

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

let authStatusBootstrapInflight: Promise<AuthStatus> | null = null;

function runAuthStatusBootstrap(loader: () => Promise<AuthStatus>): Promise<AuthStatus> {
  if (authStatusBootstrapInflight) {
    return authStatusBootstrapInflight;
  }

  authStatusBootstrapInflight = loader().finally(() => {
    authStatusBootstrapInflight = null;
  });
  return authStatusBootstrapInflight;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<AuthStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [isBootstrapped, setIsBootstrapped] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refreshStatus = useCallback(async (): Promise<AuthStatus> => {
    setLoading(true);
    try {
      const nextStatus = await getAuthStatus();
      startTransition(() => {
        setStatus(nextStatus);
        setError(null);
        setIsBootstrapped(true);
      });
      return nextStatus;
    } catch (err) {
      const message = err instanceof Error ? err.message : "加载登录状态失败";
      startTransition(() => {
        setError(message);
        setIsBootstrapped(true);
      });
      throw err;
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void runAuthStatusBootstrap(refreshStatus).catch((err) => {
      console.warn("[AuthProvider] Auth bootstrap failed:", err instanceof Error ? err.message : err);
    });
  }, [refreshStatus]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return undefined;
    }

    const handleAuthRequired = () => {
      startTransition(() => {
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
      });
    };

    window.addEventListener(AUTH_REQUIRED_EVENT, handleAuthRequired);
    return () => {
      window.removeEventListener(AUTH_REQUIRED_EVENT, handleAuthRequired);
    };
  }, []);

  const login = useCallback(async (username: string, password: string): Promise<AuthStatus> => {
    const nextStatus = await loginApi({ username, password });
    // 登录切换了用户作用域，运行时配置必须重新拉取，不能继续复用匿名或上个用户的默认 agent。
    await hydrateRuntimeOptions();
    startTransition(() => {
      setStatus(nextStatus);
      setError(null);
      setIsBootstrapped(true);
    });
    return nextStatus;
  }, []);

  const logout = useCallback(async (): Promise<AuthStatus> => {
    const nextStatus = await logoutApi();
    // 登出后同样需要重置运行时配置，避免下一个用户继续看到上个用户的主智能体配置。
    await hydrateRuntimeOptions();
    startTransition(() => {
      setStatus(nextStatus);
      setError(null);
      setIsBootstrapped(true);
    });
    return nextStatus;
  }, []);

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
