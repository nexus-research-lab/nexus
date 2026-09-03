/** Nexus Web Shell 消费的 Control owner setup 与 deployment member API。 */

import { getControlAuthBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";

import type { AuthStatus } from "./auth-api";

const CONTROL_AUTH_BASE_URL = getControlAuthBaseUrl();

export type ControlMemberRole = "owner" | "admin" | "member";
export type ControlMembershipStatus = "active" | "revoked";

export interface ControlDeploymentMember {
  deployment_id: string;
  user_id: string;
  username: string;
  display_name: string;
  role: ControlMemberRole;
  membership_status: ControlMembershipStatus;
  avatar?: string;
  last_login_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface SetupControlOwnerParams {
  setupToken: string;
  username: string;
  displayName: string;
  password: string;
  deploymentName: string;
}

export interface CreateControlMemberParams {
  username: string;
  display_name: string;
  password: string;
  role: ControlMemberRole;
}

export async function setupControlOwnerApi(
  params: SetupControlOwnerParams,
): Promise<AuthStatus> {
  return requestApi<AuthStatus>(`${CONTROL_AUTH_BASE_URL}/setup`, {
    method: "POST",
    notify_on_401: false,
    headers: { Authorization: `Bearer ${params.setupToken}` },
    body: {
      username: params.username,
      display_name: params.displayName,
      password: params.password,
      deployment_name: params.deploymentName,
    },
  });
}

export async function listControlMembersApi(): Promise<ControlDeploymentMember[]> {
  return requestApi<ControlDeploymentMember[]>(`${CONTROL_AUTH_BASE_URL}/members`, {
    method: "GET",
  });
}

export async function createControlMemberApi(
  params: CreateControlMemberParams,
): Promise<ControlDeploymentMember> {
  return requestApi<ControlDeploymentMember>(`${CONTROL_AUTH_BASE_URL}/members`, {
    method: "POST",
    body: {
      username: params.username,
      display_name: params.display_name,
      password: params.password,
      role: params.role,
    },
  });
}

export async function updateControlMemberApi(
  userID: string,
  change: { role?: ControlMemberRole; status?: ControlMembershipStatus },
): Promise<ControlDeploymentMember> {
  return requestApi<ControlDeploymentMember>(
    `${CONTROL_AUTH_BASE_URL}/members/${encodeURIComponent(userID)}`,
    { method: "PATCH", body: change },
  );
}
