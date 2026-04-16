/**
 * =====================================================
 * @File   : auth-context.ts
 * @Date   : 2026-04-07 18:24
 * @Author : leemysw
 * 2026-04-07 18:24   Create
 * =====================================================
 */

"use client";

import { createContext, useContext } from "react";

import { AuthStatus } from "@/lib/api/auth-api";

export interface AuthContextValue {
  status: AuthStatus | null;
  loading: boolean;
  is_bootstrapped: boolean;
  error: string | null;
  refresh_status: () => Promise<AuthStatus>;
  login: (username: string, password: string) => Promise<AuthStatus>;
  logout: () => Promise<AuthStatus>;
}

export const AUTH_CONTEXT = createContext<AuthContextValue | null>(null);

export function useAuth() {
  const context = useContext(AUTH_CONTEXT);
  if (!context) {
    throw new Error("useAuth must be used within AuthProvider.");
  }
  return context;
}
