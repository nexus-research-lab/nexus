import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";

export type ComputerUsePackageSource = "managed" | "environment";

export interface ComputerUsePackageStatus {
  available: boolean;
  installed: boolean;
  source?: ComputerUsePackageSource;
  version?: string;
  target_version?: string;
  protocol_version?: string;
  platform: string;
  architecture: string;
  can_install: boolean;
  can_update: boolean;
  can_remove: boolean;
  message?: string;
}

export type ComputerUseSidecarState = "stopped" | "starting" | "ready" | "stopping" | "failed";

export interface ComputerUseSidecarStatus {
  state: ComputerUseSidecarState;
  epoch: number;
  version?: string;
  protocol_version?: string;
  started_at?: string;
  unexpected_exits: number;
  last_error?: string;
}

export interface ComputerUseDoctorReport {
  package: ComputerUsePackageStatus;
  healthy: boolean;
  runtime_version?: string;
  protocol_version?: string;
  platform?: string;
  capabilities?: Record<string, unknown>;
  permissions?: Record<string, string>;
  message?: string;
}

export interface ComputerUseStatus {
  enabled: boolean;
  package: ComputerUsePackageStatus;
  sidecar: ComputerUseSidecarStatus;
  doctor?: ComputerUseDoctorReport;
  next_actions?: string[];
}

const COMPUTER_USE_API_BASE_URL = `${getAgentApiBaseUrl()}/settings/computer-use`;

export async function getComputerUseStatusApi(): Promise<ComputerUseStatus> {
  return requestApi<ComputerUseStatus>(COMPUTER_USE_API_BASE_URL, {
    method: "GET",
  });
}

export async function installComputerUseApi(): Promise<ComputerUsePackageStatus> {
  return requestApi<ComputerUsePackageStatus>(`${COMPUTER_USE_API_BASE_URL}/install`, {
    method: "POST",
  });
}

export async function updateComputerUseApi(): Promise<ComputerUsePackageStatus> {
  return requestApi<ComputerUsePackageStatus>(`${COMPUTER_USE_API_BASE_URL}/update`, {
    method: "POST",
  });
}

export async function doctorComputerUseApi(): Promise<ComputerUseDoctorReport> {
  return requestApi<ComputerUseDoctorReport>(`${COMPUTER_USE_API_BASE_URL}/doctor`, {
    method: "POST",
  });
}

export async function startComputerUseApi(): Promise<ComputerUseSidecarStatus> {
  return requestApi<ComputerUseSidecarStatus>(`${COMPUTER_USE_API_BASE_URL}/start`, {
    method: "POST",
  });
}

export async function stopComputerUseApi(): Promise<{ stopped: boolean }> {
  return requestApi<{ stopped: boolean }>(`${COMPUTER_USE_API_BASE_URL}/stop`, {
    method: "POST",
  });
}

export async function removeComputerUseApi(): Promise<{ removed: boolean }> {
  return requestApi<{ removed: boolean }>(`${COMPUTER_USE_API_BASE_URL}/runtime`, {
    method: "DELETE",
  });
}
