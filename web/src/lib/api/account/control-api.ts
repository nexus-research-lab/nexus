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

export interface ControlMemberDirectoryEntry {
  avatar?: string;
  display_name: string;
  user_id: string;
  username: string;
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

export interface ControlOrganizationInvitation {
  invitation_id: string;
  organization_id: string;
  organization_name: string;
  role: Exclude<ControlMemberRole, "owner">;
  created_by_user_id: string;
  accepted_by_user_id?: string;
  expires_at: string;
  accepted_at?: string;
  revoked_at?: string;
  created_at: string;
  updated_at: string;
}

export interface CreatedControlOrganizationInvitation extends ControlOrganizationInvitation {
  token: string;
  join_url: string;
}

export interface ControlOrganizationInvitationPreview {
  organization_name: string;
  role: Exclude<ControlMemberRole, "owner">;
  expires_at: string;
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

export async function listControlMemberDirectoryApi(): Promise<ControlMemberDirectoryEntry[]> {
  return requestApi<ControlMemberDirectoryEntry[]>(`${CONTROL_AUTH_BASE_URL}/directory/members`, {
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

export async function listControlOrganizationInvitationsApi(): Promise<ControlOrganizationInvitation[]> {
  return requestApi<ControlOrganizationInvitation[]>(`${CONTROL_AUTH_BASE_URL}/organization/invitations`, {
    method: "GET",
  });
}

export async function createControlOrganizationInvitationApi(
  role: Exclude<ControlMemberRole, "owner">,
): Promise<CreatedControlOrganizationInvitation> {
  return requestApi<CreatedControlOrganizationInvitation>(`${CONTROL_AUTH_BASE_URL}/organization/invitations`, {
    method: "POST",
    body: { role },
  });
}

export async function revokeControlOrganizationInvitationApi(invitationID: string): Promise<void> {
  await requestApi(`${CONTROL_AUTH_BASE_URL}/organization/invitations/${encodeURIComponent(invitationID)}`, {
    method: "DELETE",
  });
}

export async function deleteControlOrganizationInvitationApi(invitationID: string): Promise<void> {
  await requestApi(`${CONTROL_AUTH_BASE_URL}/organization/invitations/${encodeURIComponent(invitationID)}/record`, {
    method: "DELETE",
  });
}

export async function previewControlOrganizationInvitationApi(
  token: string,
): Promise<ControlOrganizationInvitationPreview> {
  return requestApi<ControlOrganizationInvitationPreview>(
    `${CONTROL_AUTH_BASE_URL}/organization-invitations/${encodeURIComponent(token)}`,
    { method: "GET", notify_on_401: false },
  );
}

export async function acceptControlOrganizationInvitationApi(
  token: string,
  input: { username: string; display_name: string; password: string },
): Promise<AuthStatus> {
  return requestApi<AuthStatus>(
    `${CONTROL_AUTH_BASE_URL}/organization-invitations/${encodeURIComponent(token)}/accept`,
    { method: "POST", notify_on_401: false, body: input },
  );
}
