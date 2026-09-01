import type {
  CustomMCPAuthType,
  CustomMCPSecretMap,
  CustomMCPServer,
  CustomMCPServerInput,
  CustomMCPServerType,
} from "@/types/capability/connector";

export interface CustomMCPSecretDraft {
  configured: boolean;
  key: string;
  value: string;
}

export interface CustomMCPDraft {
  args: string[];
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

export function createCustomMCPDraft(server?: CustomMCPServer): CustomMCPDraft {
  return {
    args: server?.args?.length ? [...server.args] : [],
    authType: server?.auth_type ?? "none",
    bearerToken: server?.bearer_token ?? "",
    bearerTokenConfigured:
      server?.auth_type === "bearer" && server.bearer_token == null,
    command: server?.command ?? "",
    env: secretMapToDraft(server?.env),
    headers: secretMapToDraft(server?.headers),
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
    if (draft.args.some((value) => !value.length)) return "args";
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
      args: draft.args,
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
  const query = rawQuery.trim().toLowerCase();
  if (!query) return servers;
  return servers.filter((server) => [
    server.connector_id,
    server.configuration_state,
    server.name,
    server.type,
    server.command ?? "",
    server.url ?? "",
  ].some((value) => value.toLowerCase().includes(query)));
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
  values?: CustomMCPSecretMap,
): CustomMCPSecretDraft[] {
  return Object.keys(values ?? {}).sort().map((key) => ({
    configured: values?.[key] === null,
    key,
    value: values?.[key] ?? "",
  }));
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
