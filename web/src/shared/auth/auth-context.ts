// INPUT: 认证快照、独立多人部署能力和 Context。
// OUTPUT: 认证消费 Hook 与在线 Team 准入判断。
// POS: 共享认证投影；不以组织身份推断多人部署，不负责请求。
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

import { AuthStatus } from "@/lib/api/account/auth-api";

export interface AuthContextValue {
  status: AuthStatus | null;
  loading: boolean;
  isBootstrapped: boolean;
  error: string | null;
  refreshStatus: () => Promise<AuthStatus>;
  login: (username: string, password: string) => Promise<AuthStatus>;
  logout: () => Promise<AuthStatus>;
}

export const AUTH_CONTEXT = createContext<AuthContextValue | null>(null);

export function isRemoteAccountAuthenticated(status: AuthStatus | null): boolean {
  return status?.authenticated === true && status.auth_method === "password";
}

export function hasTeamAccess(status: AuthStatus | null): boolean {
  return status?.multiplayer_enabled === true && isRemoteAccountAuthenticated(status) && Boolean(status.organization_id);
}

export function useAuth() {
  const context = useContext(AUTH_CONTEXT);
  if (!context) {
    throw new Error("useAuth must be used within AuthProvider.");
  }
  return context;
}
