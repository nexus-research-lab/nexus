/**
 * =====================================================
 * @File   : auth-api.ts
 * @Date   : 2026-04-07 18:24
 * @Author : leemysw
 * 2026-04-07 18:24   Create
 * =====================================================
 */

import { get_agent_api_base_url } from "@/config/options";
import { request_api } from "@/lib/api/http";

const AUTH_API_BASE_URL = get_agent_api_base_url();

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

export async function get_auth_status(): Promise<AuthStatus> {
  return request_api<AuthStatus>(`${AUTH_API_BASE_URL}/auth/status`, {
    method: "GET",
    notify_on_401: false,
  });
}

export async function login_api(params: LoginParams): Promise<AuthStatus> {
  return request_api<AuthStatus>(`${AUTH_API_BASE_URL}/auth/login`, {
    method: "POST",
    notify_on_401: false,
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(params),
  });
}

export async function logout_api(): Promise<AuthStatus> {
  return request_api<AuthStatus>(`${AUTH_API_BASE_URL}/auth/logout`, {
    method: "POST",
    notify_on_401: false,
  });
}
