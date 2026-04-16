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
  useState,
} from "react";

import { AuthStatus, get_auth_status, login_api, logout_api } from "@/lib/api/auth-api";
import { AUTH_REQUIRED_EVENT } from "@/lib/api/http";
import { AUTH_CONTEXT } from "@/shared/auth/auth-context";

const DEFAULT_UNAUTHORIZED_STATUS: AuthStatus = {
  auth_required: true,
  password_login_enabled: true,
  authenticated: false,
  username: null,
};

export function AuthProvider({ children }: { children: ReactNode }) {
  const [status, set_status] = useState<AuthStatus | null>(null);
  const [loading, set_loading] = useState(true);
  const [is_bootstrapped, set_is_bootstrapped] = useState(false);
  const [error, set_error] = useState<string | null>(null);

  const refresh_status = useCallback(async (): Promise<AuthStatus> => {
    set_loading(true);
    try {
      const next_status = await get_auth_status();
      startTransition(() => {
        set_status(next_status);
        set_error(null);
        set_is_bootstrapped(true);
      });
      return next_status;
    } catch (err) {
      const message = err instanceof Error ? err.message : "加载登录状态失败";
      startTransition(() => {
        set_error(message);
        set_is_bootstrapped(true);
      });
      throw err;
    } finally {
      set_loading(false);
    }
  }, []);

  useEffect(() => {
    void refresh_status().catch(() => undefined);
  }, [refresh_status]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return undefined;
    }

    const handle_auth_required = () => {
      startTransition(() => {
        set_is_bootstrapped(true);
        set_status((current_status) => {
          if (!current_status) {
            return DEFAULT_UNAUTHORIZED_STATUS;
          }
          return {
            ...current_status,
            authenticated: false,
            username: null,
          };
        });
      });
    };

    window.addEventListener(AUTH_REQUIRED_EVENT, handle_auth_required);
    return () => {
      window.removeEventListener(AUTH_REQUIRED_EVENT, handle_auth_required);
    };
  }, []);

  const login = useCallback(async (username: string, password: string): Promise<AuthStatus> => {
    const next_status = await login_api({ username, password });
    startTransition(() => {
      set_status(next_status);
      set_error(null);
      set_is_bootstrapped(true);
    });
    return next_status;
  }, []);

  const logout = useCallback(async (): Promise<AuthStatus> => {
    const next_status = await logout_api();
    startTransition(() => {
      set_status(next_status);
      set_error(null);
      set_is_bootstrapped(true);
    });
    return next_status;
  }, []);

  return (
    <AUTH_CONTEXT.Provider
      value={{
        status,
        loading,
        is_bootstrapped,
        error,
        refresh_status,
        login,
        logout,
      }}
    >
      {children}
    </AUTH_CONTEXT.Provider>
  );
}
