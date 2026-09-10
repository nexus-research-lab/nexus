/** 认证、个人资料、密码与个人用量的 HTTP 边界。 */

import { isDesktopRuntime } from "@/config/desktop-runtime";
import { getAgentApiBaseUrl, getControlAuthBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";

const AUTH_API_BASE_URL = getAgentApiBaseUrl();
const CONTROL_AUTH_BASE_URL = getControlAuthBaseUrl();

export interface AuthStatus {
  auth_required: boolean;
  password_login_enabled: boolean;
  authenticated: boolean;
  username: string | null;
  user_id?: string | null;
  display_name?: string | null;
  role?: string | null;
  avatar?: string | null;
  auth_method?: string | null;
  setup_required?: boolean;
  setup_enabled?: boolean;
}

export interface LoginParams {
  username: string;
  password: string;
}

export interface DailyTokenUsage {
  date: string;
  input_tokens: number;
  output_tokens: number;
  cache_tokens: number;
  total_tokens: number;
}

export interface TokenUsageSummary {
  daily?: DailyTokenUsage[];
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

interface PersonalSubscriptionSummary {
  plan_key: string;
  plan_name: string;
  monthly_token_limit: number | null;
  used_tokens: number;
  used_percent: number | null;
  period_start: string;
  period_end: string;
}

export interface PersonalProfile {
  user: {
    user_id: string;
    username: string;
    display_name: string;
    role: string;
    avatar: string;
    auth_method: string;
  };
  token_usage: TokenUsageSummary;
  subscription?: PersonalSubscriptionSummary | null;
  can_change_password: boolean;
  can_update_profile: boolean;
}

export interface ChangePasswordParams {
  request_id: string;
  current_password: string;
  new_password: string;
}

export interface PasswordChangeReceipt {
  request_id: string;
  effect: "committed" | "not_applied" | "unknown";
}

export interface UpdatePersonalProfileParams {
  avatar?: string;
  authMethod?: string;
}

export async function getAuthStatus(): Promise<AuthStatus> {
  const localStatus = await requestApi<AuthStatus>(`${AUTH_API_BASE_URL}/auth/status`, {
    method: "GET",
    notify_on_401: false,
  });
  if (!isDesktopRuntime()) {
    return localStatus;
  }
  try {
    const remoteStatus = await requestApi<AuthStatus>(`${CONTROL_AUTH_BASE_URL}/status`, {
      method: "GET",
      notify_on_401: false,
    });
    if (!remoteStatus.authenticated) {
      return {
        ...localStatus,
        password_login_enabled: remoteStatus.password_login_enabled,
      };
    }
    return {
      ...remoteStatus,
      auth_required: false,
      setup_enabled: false,
      setup_required: false,
      user_id: localStatus.user_id,
    };
  } catch {
    return {
      ...localStatus,
      password_login_enabled: true,
    };
  }
}

export async function loginApi(params: LoginParams): Promise<AuthStatus> {
  const status = await requestApi<AuthStatus>(`${CONTROL_AUTH_BASE_URL}/login`, {
    method: "POST",
    notify_on_401: false,
    body: JSON.stringify(params),
  });
  return isDesktopRuntime() ? getAuthStatus() : status;
}

export async function logoutApi(): Promise<AuthStatus> {
  const status = await requestApi<AuthStatus>(`${CONTROL_AUTH_BASE_URL}/logout`, {
    method: "POST",
    notify_on_401: false,
  });
  return isDesktopRuntime() ? getAuthStatus() : status;
}

export async function getPersonalProfileApi(): Promise<PersonalProfile> {
  const profile = await requestApi<PersonalProfile>(`${AUTH_API_BASE_URL}/settings/profile`, {
    method: "GET",
  });
  if (!isDesktopRuntime()) {
    return profile;
  }
  const status = await getAuthStatus();
  if (status.auth_method !== "password") {
    return profile;
  }
  return {
    ...profile,
    can_change_password: true,
    can_update_profile: true,
    user: {
      ...profile.user,
      auth_method: status.auth_method,
      avatar: status.avatar ?? "",
      display_name: status.display_name ?? status.username ?? "",
      role: status.role ?? profile.user.role,
      username: status.username ?? "",
    },
  };
}

export async function updatePersonalProfileApi(params: UpdatePersonalProfileParams): Promise<void> {
  const endpoint = isDesktopRuntime() && params.authMethod !== "password"
    ? `${AUTH_API_BASE_URL}/settings/profile`
    : `${CONTROL_AUTH_BASE_URL}/profile`;
  await requestApi<unknown>(endpoint, {
    method: "PATCH",
    body: {
      avatar: params.avatar ?? "",
    },
  });
}

export async function changePasswordApi(params: ChangePasswordParams): Promise<void> {
  await requestApi<unknown>(`${CONTROL_AUTH_BASE_URL}/profile/password`, {
    method: "POST",
    body: {
      request_id: params.request_id,
      current_password: params.current_password,
      new_password: params.new_password,
    },
  });
}

export async function getPasswordChangeReceiptApi(
  requestID: string,
): Promise<PasswordChangeReceipt> {
  const query = new URLSearchParams({ request_id: requestID });
  return requestApi<PasswordChangeReceipt>(
    `${CONTROL_AUTH_BASE_URL}/profile/password/receipt?${query.toString()}`,
    { method: "GET" },
  );
}

export async function settlePasswordChangeNotAppliedApi(
  requestID: string,
): Promise<PasswordChangeReceipt> {
  return requestApi<PasswordChangeReceipt>(
    `${CONTROL_AUTH_BASE_URL}/profile/password/receipt/not-applied`,
    {
      method: "POST",
      body: { request_id: requestID },
    },
  );
}
