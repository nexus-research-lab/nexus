/**
 * INPUT: Operation Stage snapshot, presence, and Navi target URL requests.
 * OUTPUT: API calls plus a same-origin iframe URL for the controlled browser-page endpoint.
 * POS: Frontend transport boundary; browser models keep display URLs separate from proxy URLs.
 */
import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";
import { ApiRequestError } from "@/lib/api/core/http-error";

import type { NexusOperationSnapshot } from "./operation-types";

const OPERATION_STAGE_API_BASE_URL = `${getAgentApiBaseUrl()}/operation/stage`;
const OPERATION_BROWSER_API_BASE_URL = `${getAgentApiBaseUrl()}/operation/browser`;

interface OperationStageSnapshotEnvelope {
  key: string;
  snapshot: NexusOperationSnapshot | null;
  updated_at: string;
}

interface OperationStagePresenceEnvelope {
  session_key: string;
  client_id: string;
  active: boolean;
  expires_at?: string;
}

export function getOperationBrowserPageUrl(url: string): string {
  const query = new URLSearchParams({ url });
  return `${OPERATION_BROWSER_API_BASE_URL}/page?${query.toString()}`;
}

export async function getOperationStageSnapshotApi(
  key: string,
): Promise<NexusOperationSnapshot | null> {
  const query = new URLSearchParams({ key });
  try {
    const result = await requestApi<OperationStageSnapshotEnvelope>(
      `${OPERATION_STAGE_API_BASE_URL}/snapshot?${query.toString()}`,
      {
        method: "GET",
        notify_on_401: false,
        timeout_ms: 6000,
      },
    );
    return result.snapshot ?? null;
  } catch (error) {
    if (error instanceof ApiRequestError && error.status === 404) {
      return null;
    }
    return null;
  }
}

export async function saveOperationStageSnapshotApi(
  key: string,
  snapshot: NexusOperationSnapshot,
): Promise<void> {
  try {
    await requestApi<OperationStageSnapshotEnvelope>(
      `${OPERATION_STAGE_API_BASE_URL}/snapshot`,
      {
        method: "PUT",
        notify_on_401: false,
        timeout_ms: 8000,
        body: {
          key,
          snapshot,
        },
      },
    );
  } catch {
    // 舞台快照是恢复体验兜底，失败不能打断主会话流。
  }
}

export async function touchOperationStagePresenceApi(
  session_key: string,
  client_id: string,
): Promise<void> {
  try {
    await requestApi<OperationStagePresenceEnvelope>(
      `${OPERATION_STAGE_API_BASE_URL}/presence`,
      {
        method: "PUT",
        notify_on_401: false,
        timeout_ms: 4000,
        body: { session_key, client_id },
      },
    );
  } catch {
    // Presence is an ephemeral routing hint; a failed heartbeat must not break the stage UI.
  }
}

export async function closeOperationStagePresenceApi(
  session_key: string,
  client_id: string,
): Promise<void> {
  try {
    await requestApi<OperationStagePresenceEnvelope>(
      `${OPERATION_STAGE_API_BASE_URL}/presence`,
      {
        method: "DELETE",
        notify_on_401: false,
        timeout_ms: 3000,
        body: { session_key, client_id },
      },
    );
  } catch {
    // The backend TTL closes abandoned clients when unload requests are cancelled.
  }
}
