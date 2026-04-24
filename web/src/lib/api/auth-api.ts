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
  user_id?: string | null;
  display_name?: string | null;
  role?: string | null;
  auth_method?: string | null;
  setup_required?: boolean;
  access_token_enabled?: boolean;
}

export interface LoginParams {
  username: string;
  password: string;
}

export interface TokenUsageSummary {
  input_tokens: number;
  output_tokens: number;
  cache_creation_input_tokens: number;
  cache_read_input_tokens: number;
  total_tokens: number;
  quota_limit_tokens: number | null;
  session_count: number;
  message_count: number;
  updated_at: string;
}

export interface PersonalProfile {
  user: {
    user_id: string;
    username: string;
    display_name: string;
    role: string;
    auth_method: string;
  };
  token_usage: TokenUsageSummary;
  can_change_password: boolean;
}

export interface ChangePasswordParams {
  current_password: string;
  new_password: string;
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
    body: JSON.stringify(params),
  });
}

export async function logout_api(): Promise<AuthStatus> {
  return request_api<AuthStatus>(`${AUTH_API_BASE_URL}/auth/logout`, {
    method: "POST",
    notify_on_401: false,
  });
}

export async function get_personal_profile_api(): Promise<PersonalProfile> {
  return request_api<PersonalProfile>(`${AUTH_API_BASE_URL}/settings/profile`, {
    method: "GET",
  });
}

export async function change_password_api(params: ChangePasswordParams): Promise<AuthStatus> {
  return request_api<AuthStatus>(`${AUTH_API_BASE_URL}/settings/profile/password`, {
    method: "POST",
    body: {
      current_password: params.current_password,
      new_password: params.new_password,
    },
  });
}
