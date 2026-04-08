/**
 * =====================================================
 * @File   : auth-api.ts
 * @Date   : 2026-04-07 18:24
 * @Author : leemysw
 * 2026-04-07 18:24   Create
 * =====================================================
 */

import { getAgentApiBaseUrl } from "@/config/options";
import { request_api } from "@/lib/http";

const AUTH_API_BASE_URL = getAgentApiBaseUrl();

export interface AuthStatus {
  auth_required: boolean;
  password_login_enabled: boolean;
  authenticated: boolean;
  username: string | null;
}

export interface LoginParams {
  username: string;
  password: string;
}

export async function getAuthStatus(): Promise<AuthStatus> {
  return request_api<AuthStatus>(`${AUTH_API_BASE_URL}/auth/status`, {
    method: "GET",
    notify_on_401: false,
  });
}

export async function loginApi(params: LoginParams): Promise<AuthStatus> {
  return request_api<AuthStatus>(`${AUTH_API_BASE_URL}/auth/login`, {
    method: "POST",
    notify_on_401: false,
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(params),
  });
}

export async function logoutApi(): Promise<AuthStatus> {
  return request_api<AuthStatus>(`${AUTH_API_BASE_URL}/auth/logout`, {
    method: "POST",
    notify_on_401: false,
  });
}
