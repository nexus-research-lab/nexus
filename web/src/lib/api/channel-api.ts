import { getAgentApiBaseUrl } from "@/config/options";
import { requestApi } from "@/lib/api/http";

const AGENT_API_BASE_URL = getAgentApiBaseUrl();
const CHANNEL_API_BASE_URL = `${AGENT_API_BASE_URL}/capability`;

export type ImChannelType =
  | "dingtalk"
  | "wechat"
  | "weixin-personal"
  | "feishu"
  | "telegram"
  | "discord";
export type ImPairingStatus = "pending" | "active" | "disabled" | "rejected";
export type ImChatType = "dm" | "group";
type ImChannelCapability =
  | "text"
  | "media"
  | "typing"
  | "thread"
  | "reply"
  | "receipt"
  | "durable_final";

export interface ChannelCredentialField {
  key: string;
  label: string;
  kind: string;
  required: boolean;
  secret: boolean;
  placeholder?: string;
}

interface ChannelCatalogItem {
  channel_type: ImChannelType;
  title: string;
  bot_label: string;
  description: string;
  docs_url?: string;
  runtime_status: "ready" | "external_adapter" | string;
  runtime_note?: string;
  supports_group: boolean;
  supports_qr_code: boolean;
  supports_oauth_link: boolean;
  capabilities: ImChannelCapability[];
  credential_fields: ChannelCredentialField[];
}

interface ChannelStats {
  paired_user_count: number;
  paired_group_count: number;
  pending_count: number;
}

export interface ChannelAccountView {
  account_id: string;
  user_id?: string;
  status: string;
  last_error?: string;
  created_at: string;
  updated_at: string;
}

export interface ChannelConfigView extends ChannelCatalogItem {
  configured: boolean;
  connection_state: string;
  status: string;
  agent_id?: string;
  agent_name?: string;
  public_config?: Record<string, string>;
  has_credentials: boolean;
  last_error?: string;
  qr_payload?: string;
  updated_at?: string;
  stats: ChannelStats;
  accounts?: ChannelAccountView[];
}

export interface UpsertChannelConfigPayload {
  agent_id: string;
  config?: Record<string, string>;
  credentials?: Record<string, string>;
}

type ChannelLoginStatus = "running" | "verify_code_required" | "succeeded" | "error" | "expired" | "cancelled";

export interface ChannelLoginView {
  login_id: string;
  channel_type: ImChannelType;
  status: ChannelLoginStatus | string;
  command?: string;
  qr_payload?: string;
  qr_payload_type?: string;
  output?: string;
  error?: string;
  account_id?: string;
  user_id?: string;
  verify_code_hint?: string;
  started_at: string;
  updated_at: string;
  finished_at?: string;
}

export interface PairingView {
  pairing_id: string;
  channel_type: ImChannelType;
  account_id?: string;
  chat_type: ImChatType;
  external_ref: string;
  thread_id?: string;
  session_key: string;
  external_name?: string;
  agent_id: string;
  agent_name?: string;
  status: ImPairingStatus;
  source: string;
  last_message_at?: string;
  created_at: string;
  updated_at: string;
}

export interface ListPairingsParams {
  channel_type?: ImChannelType | "";
  status?: ImPairingStatus | "";
  agent_id?: string;
}

export interface CreatePairingPayload {
  channel_type: ImChannelType;
  account_id?: string;
  chat_type: ImChatType;
  external_ref: string;
  thread_id?: string;
  external_name?: string;
  agent_id: string;
  status?: ImPairingStatus;
  source?: string;
}

export interface UpdatePairingPayload {
  status?: ImPairingStatus;
  agent_id?: string;
  external_name?: string;
}

function buildQuery(params?: Record<string, string | undefined>): string {
  const searchParams = new URLSearchParams();
  Object.entries(params ?? {}).forEach(([key, value]) => {
    if (value && value.trim()) {
      searchParams.set(key, value);
    }
  });
  const query = searchParams.toString();
  return query ? `?${query}` : "";
}

export async function listChannelsApi(): Promise<ChannelConfigView[]> {
  return requestApi<ChannelConfigView[]>(`${CHANNEL_API_BASE_URL}/channels`, {
    method: "GET",
  });
}

export async function upsertChannelConfigApi(
  channelType: ImChannelType,
  payload: UpsertChannelConfigPayload,
): Promise<ChannelConfigView> {
  return requestApi<ChannelConfigView>(
    `${CHANNEL_API_BASE_URL}/channels/${encodeURIComponent(channelType)}/config`,
    {
      method: "PUT",
      body: JSON.stringify({
        agent_id: payload.agent_id,
        config: payload.config ?? {},
        credentials: payload.credentials ?? {},
      }),
    },
  );
}

export async function deleteChannelConfigApi(
  channelType: ImChannelType,
): Promise<{ configured: boolean }> {
  return requestApi<{ configured: boolean }>(
    `${CHANNEL_API_BASE_URL}/channels/${encodeURIComponent(channelType)}/config`,
    {
      method: "DELETE",
    },
  );
}

export async function deleteChannelAccountApi(
  channelType: ImChannelType,
  accountId: string,
): Promise<ChannelConfigView> {
  return requestApi<ChannelConfigView>(
    `${CHANNEL_API_BASE_URL}/channels/${encodeURIComponent(channelType)}/accounts/${encodeURIComponent(accountId)}`,
    {
      method: "DELETE",
    },
  );
}

export async function startChannelLoginApi(
  channelType: ImChannelType,
): Promise<ChannelLoginView> {
  return requestApi<ChannelLoginView>(
    `${CHANNEL_API_BASE_URL}/channels/${encodeURIComponent(channelType)}/login`,
    {
      method: "POST",
    },
  );
}

export async function getChannelLoginApi(
  channelType: ImChannelType,
  loginId: string,
): Promise<ChannelLoginView> {
  return requestApi<ChannelLoginView>(
    `${CHANNEL_API_BASE_URL}/channels/${encodeURIComponent(channelType)}/login/${encodeURIComponent(loginId)}`,
    {
      method: "GET",
    },
  );
}

export async function submitChannelLoginVerifyCodeApi(
  channelType: ImChannelType,
  loginId: string,
  verifyCode: string,
): Promise<ChannelLoginView> {
  return requestApi<ChannelLoginView>(
    `${CHANNEL_API_BASE_URL}/channels/${encodeURIComponent(channelType)}/login/${encodeURIComponent(loginId)}/verify-code`,
    {
      method: "POST",
      body: JSON.stringify({ verify_code: verifyCode }),
    },
  );
}

export async function listPairingsApi(
  params: ListPairingsParams = {},
): Promise<PairingView[]> {
  return requestApi<PairingView[]>(
    `${CHANNEL_API_BASE_URL}/pairings${buildQuery({
      channel_type: params.channel_type || undefined,
      status: params.status || undefined,
      agent_id: params.agent_id,
    })}`,
    {
      method: "GET",
    },
  );
}

export async function createPairingApi(
  payload: CreatePairingPayload,
): Promise<PairingView> {
  return requestApi<PairingView>(`${CHANNEL_API_BASE_URL}/pairings`, {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export async function updatePairingApi(
  pairingId: string,
  payload: UpdatePairingPayload,
): Promise<PairingView> {
  return requestApi<PairingView>(
    `${CHANNEL_API_BASE_URL}/pairings/${encodeURIComponent(pairingId)}`,
    {
      method: "PATCH",
      body: JSON.stringify(payload),
    },
  );
}

export async function deletePairingApi(
  pairingId: string,
): Promise<{ success: boolean }> {
  return requestApi<{ success: boolean }>(
    `${CHANNEL_API_BASE_URL}/pairings/${encodeURIComponent(pairingId)}`,
    {
      method: "DELETE",
    },
  );
}
