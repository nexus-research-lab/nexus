import { get_agent_api_base_url } from "@/config/options";
import { request_api } from "@/lib/api/http";

const AGENT_API_BASE_URL = get_agent_api_base_url();
const CHANNEL_API_BASE_URL = `${AGENT_API_BASE_URL}/capability`;

export type ImChannelType = "dingtalk" | "wechat" | "feishu" | "telegram" | "discord";
export type ImPairingStatus = "pending" | "active" | "disabled" | "rejected";
export type ImChatType = "dm" | "group";

export interface ChannelCredentialField {
  key: string;
  label: string;
  kind: string;
  required: boolean;
  secret: boolean;
  placeholder?: string;
}

export interface ChannelCatalogItem {
  channel_type: ImChannelType;
  title: string;
  bot_label: string;
  description: string;
  docs_url?: string;
  supports_group: boolean;
  supports_qr_code: boolean;
  supports_oauth_link: boolean;
  credential_fields: ChannelCredentialField[];
}

export interface ChannelStats {
  paired_user_count: number;
  paired_group_count: number;
  pending_count: number;
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
}

export interface UpsertChannelConfigPayload {
  agent_id: string;
  config?: Record<string, string>;
  credentials?: Record<string, string>;
}

export interface PairingView {
  pairing_id: string;
  channel_type: ImChannelType;
  chat_type: ImChatType;
  external_ref: string;
  thread_id?: string;
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

function build_query(params?: Record<string, string | undefined>): string {
  const search_params = new URLSearchParams();
  Object.entries(params ?? {}).forEach(([key, value]) => {
    if (value && value.trim()) {
      search_params.set(key, value);
    }
  });
  const query = search_params.toString();
  return query ? `?${query}` : "";
}

export async function list_channels_api(): Promise<ChannelConfigView[]> {
  return request_api<ChannelConfigView[]>(`${CHANNEL_API_BASE_URL}/channels`, {
    method: "GET",
  });
}

export async function upsert_channel_config_api(
  channel_type: ImChannelType,
  payload: UpsertChannelConfigPayload,
): Promise<ChannelConfigView> {
  return request_api<ChannelConfigView>(
    `${CHANNEL_API_BASE_URL}/channels/${encodeURIComponent(channel_type)}/config`,
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

export async function delete_channel_config_api(
  channel_type: ImChannelType,
): Promise<{ configured: boolean }> {
  return request_api<{ configured: boolean }>(
    `${CHANNEL_API_BASE_URL}/channels/${encodeURIComponent(channel_type)}/config`,
    {
      method: "DELETE",
    },
  );
}

export async function list_pairings_api(
  params: ListPairingsParams = {},
): Promise<PairingView[]> {
  return request_api<PairingView[]>(
    `${CHANNEL_API_BASE_URL}/pairings${build_query({
      channel_type: params.channel_type || undefined,
      status: params.status || undefined,
      agent_id: params.agent_id,
    })}`,
    {
      method: "GET",
    },
  );
}

export async function create_pairing_api(
  payload: CreatePairingPayload,
): Promise<PairingView> {
  return request_api<PairingView>(`${CHANNEL_API_BASE_URL}/pairings`, {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export async function update_pairing_api(
  pairing_id: string,
  payload: UpdatePairingPayload,
): Promise<PairingView> {
  return request_api<PairingView>(
    `${CHANNEL_API_BASE_URL}/pairings/${encodeURIComponent(pairing_id)}`,
    {
      method: "PATCH",
      body: JSON.stringify(payload),
    },
  );
}

export async function delete_pairing_api(
  pairing_id: string,
): Promise<{ success: boolean }> {
  return request_api<{ success: boolean }>(
    `${CHANNEL_API_BASE_URL}/pairings/${encodeURIComponent(pairing_id)}`,
    {
      method: "DELETE",
    },
  );
}
