// INPUT: 脱敏的自定义 MCP 快照、视图提供的本地身份与当前表单草稿。
// OUTPUT: 带稳定本地行身份的草稿、现有配置校验和不含行身份的协议输入。
// POS: Custom MCP 草稿/目录投影；秘密 null 保留语义归配置协议，不恢复原始秘密。

import { createUiSearchMatcher } from "@/shared/ui/form/search-query";
import type {
  CustomMCPAuthType,
  CustomMCPSecretMap,
  CustomMCPServer,
  CustomMCPServerInput,
  CustomMCPServerType,
} from "@/types/capability/connector";

export interface CustomMCPArgumentDraft {
  id: string;
  value: string;
}

export interface CustomMCPSecretDraft {
  configured: boolean;
  id: string;
  key: string;
  value: string;
}

export interface CustomMCPDraft {
  args: CustomMCPArgumentDraft[];
  authType: CustomMCPAuthType;
  bearerToken: string;
  bearerTokenConfigured: boolean;
  command: string;
  env: CustomMCPSecretDraft[];
  headers: CustomMCPSecretDraft[];
  name: string;
  type: CustomMCPServerType;
  url: string;
}

export type CustomMCPDraftError =
  | "args"
  | "bearer_token"
  | "command"
  | "env"
  | "headers"
  | "name"
  | "url";

export function createCustomMCPArgumentDraft(id: string, value = ""): CustomMCPArgumentDraft {
  return { id, value };
}

export function createCustomMCPSecretDraft(
  id: string,
  key = "",
  value: string | null = "",
): CustomMCPSecretDraft {
  return { configured: value === null, id, key, value: value ?? "" };
}

export function createCustomMCPDraft(server: CustomMCPServer | undefined, draftId: string): CustomMCPDraft {
  return {
    args: server?.args?.map((value, index) => createCustomMCPArgumentDraft(`${draftId}-arg-${index}`, value)) ?? [],
    authType: server?.auth_type ?? "none",
    bearerToken: server?.bearer_token ?? "",
    bearerTokenConfigured:
      server?.auth_type === "bearer" && server.bearer_token == null,
    command: server?.command ?? "",
    env: secretMapToDraft(server?.env, `${draftId}-env`),
    headers: secretMapToDraft(server?.headers, `${draftId}-header`),
    name: server?.name ?? "",
    type: server?.type ?? "stdio",
    url: server?.url ?? "",
  };
}

export function validateCustomMCPDraft(
  draft: CustomMCPDraft,
): CustomMCPDraftError | null {
  if (!/^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/.test(draft.name.trim())) {
    return "name";
  }
  if (draft.type === "stdio") {
    if (!draft.command.trim()) return "command";
    if (draft.args.some((row) => !row.value.length)) return "args";
    return validateSecretRows(draft.env) ? "env" : null;
  }
  try {
    const parsed = new URL(draft.url.trim());
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
      return "url";
    }
  } catch {
    return "url";
  }
  if (
    draft.authType === "bearer"
    && !draft.bearerTokenConfigured
    && !draft.bearerToken.trim()
  ) {
    return "bearer_token";
  }
  if (draft.authType === "headers" && validateSecretRows(draft.headers)) {
    return "headers";
  }
  return null;
}

export function buildCustomMCPServerInput(
  draft: CustomMCPDraft,
): CustomMCPServerInput {
  const base = {
    name: draft.name.trim(),
    type: draft.type,
  } as const;
  if (draft.type === "stdio") {
    return {
      ...base,
      command: draft.command.trim(),
      args: draft.args.map((row) => row.value),
      env: secretDraftToMap(draft.env),
    };
  }
  return {
    ...base,
    url: draft.url.trim(),
    auth_type: draft.authType,
    bearer_token: draft.authType === "bearer"
      ? draft.bearerTokenConfigured && !draft.bearerToken
        ? null
        : draft.bearerToken.trim()
      : undefined,
    headers: draft.authType === "headers"
      ? secretDraftToMap(draft.headers)
      : undefined,
  };
}

export function filterCustomMCPServers(
  servers: CustomMCPServer[],
  rawQuery: string,
): CustomMCPServer[] {
  const search = createUiSearchMatcher(rawQuery);
  return servers.filter((server) => search.matches([
    server.connector_id,
    server.configuration_state,
    server.name,
    server.type,
    server.command,
    server.url,
  ]));
}

export function isCustomMCPConnectorId(
  connectorId: string | null | undefined,
): boolean {
  return connectorId?.startsWith("custom-mcp:") ?? false;
}

export function getCustomMCPConnectionTarget(
  server: CustomMCPServer,
): string {
  if (isCustomMCPRecoveryRequired(server)) return "";
  if (server.type !== "stdio") {
    return server.url?.trim() ?? "";
  }
  return [server.command?.trim(), ...(server.args ?? [])]
    .filter(Boolean)
    .join(" ");
}

export function isCustomMCPRecoveryRequired(
  server: CustomMCPServer,
): boolean {
  return server.configuration_state === "recovery_required";
}

export function getCustomMCPRecoveryIdentity(
  server: CustomMCPServer,
): string {
  return server.connector_id.slice(-8);
}

export function getCustomMCPDisplayName(
  server: CustomMCPServer,
  recoveryLabel: string,
): string {
  return isCustomMCPRecoveryRequired(server)
    ? `${recoveryLabel} · ${getCustomMCPRecoveryIdentity(server)}`
    : server.name;
}

function secretMapToDraft(
  values: CustomMCPSecretMap | undefined,
  idPrefix: string,
): CustomMCPSecretDraft[] {
  if (!values) return [];
  return Object.keys(values).sort().map((key, index) => (
    createCustomMCPSecretDraft(`${idPrefix}-${index}`, key, values[key])
  ));
}

function secretDraftToMap(rows: CustomMCPSecretDraft[]): CustomMCPSecretMap {
  return Object.fromEntries(rows.map((row) => [
    row.key.trim(),
    row.configured && !row.value ? null : row.value,
  ]));
}

function validateSecretRows(rows: CustomMCPSecretDraft[]): boolean {
  const keys = rows.map((row) => row.key.trim());
  return keys.some((key) => !key)
    || new Set(keys).size !== keys.length
    || rows.some((row) => !row.configured && !row.value);
}
