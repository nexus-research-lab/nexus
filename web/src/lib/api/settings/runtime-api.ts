import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";
import type {
  NXSRuntimeStatus,
  CUAInstallationStatus,
  SandboxResourceInspection,
  SandboxResourceReconcileResult,
} from "@/types/settings/preferences";

const NXS_RUNTIME_STATUS_API_URL = `${getAgentApiBaseUrl()}/settings/runtime/nxs/status`;
const SANDBOX_RESOURCES_API_URL = `${getAgentApiBaseUrl()}/settings/runtime/sandbox/resources`;
const SANDBOX_RECONCILE_API_URL = `${getAgentApiBaseUrl()}/settings/runtime/sandbox/reconcile`;
const COMPUTER_USE_API_URL = `${getAgentApiBaseUrl()}/settings/runtime/computer-use`;

export async function getNxsRuntimeStatusApi(includeSandbox = false): Promise<NXSRuntimeStatus> {
  return requestApi<NXSRuntimeStatus>(includeSandbox ? `${NXS_RUNTIME_STATUS_API_URL}?include_sandbox=true` : NXS_RUNTIME_STATUS_API_URL, {
    method: "GET",
    timeout_ms: 8_000,
  });
}

export async function inspectSandboxResourcesApi(): Promise<SandboxResourceInspection> {
  return requestApi<SandboxResourceInspection>(SANDBOX_RESOURCES_API_URL, {
    method: "GET",
    timeout_ms: 8_000,
  });
}

export async function reconcileSandboxResourcesApi(input: {
  older_than_seconds: number;
  apply: boolean;
}): Promise<SandboxResourceReconcileResult> {
  return requestApi<SandboxResourceReconcileResult>(SANDBOX_RECONCILE_API_URL, {
    method: "POST",
    body: input,
    timeout_ms: 8_000,
  });
}

export async function getComputerUseStatusApi(): Promise<CUAInstallationStatus> {
  return requestApi<CUAInstallationStatus>(COMPUTER_USE_API_URL, {
    method: "GET",
    timeout_ms: 8_000,
  });
}

export async function installComputerUseApi(): Promise<CUAInstallationStatus> {
  return requestApi<CUAInstallationStatus>(`${COMPUTER_USE_API_URL}/install`, {
    method: "POST",
    timeout_ms: 15 * 60_000,
  });
}

export async function setComputerUseEnabledApi(enabled: boolean): Promise<CUAInstallationStatus> {
  return requestApi<CUAInstallationStatus>(`${COMPUTER_USE_API_URL}/${enabled ? "enable" : "disable"}`, {
    method: "POST",
    timeout_ms: 8_000,
  });
}

export async function checkComputerUsePermissionsApi(): Promise<CUAInstallationStatus> {
  return requestApi<CUAInstallationStatus>(`${COMPUTER_USE_API_URL}/permissions/check`, {
    method: "POST",
    timeout_ms: 15_000,
  });
}
