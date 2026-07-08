type DesktopBridgeKind =
  | "app.get_app_version"
  | "app.open_external_url"
  | "app.export_logs"
  | "app.open_route"
  | "app.get_persistent_state"
  | "app.set_persistent_state"
  | "app.remove_persistent_state"
  | "app.get_global_shortcut_status"
  | "app.set_global_shortcut_enabled"
  | "app.set_global_shortcut_accelerator"
  | "app.reset_global_shortcut_accelerator";

interface DesktopBridgeRequest<TPayload = Record<string, unknown>> {
  schema_version: 1;
  request_id?: string;
  kind: DesktopBridgeKind;
  payload?: TPayload;
}

export interface DesktopAppVersion {
  app_mode: string;
  app_version: string;
  build_number: string;
  platform: string;
}

export interface DesktopExportLogsResult {
  cancelled: boolean;
  path?: string;
}

export interface DesktopPersistentStateResult {
  key: string;
  value?: string | null;
}

interface NativeDesktopBridge {
  invoke<TPayload, TResult>(message: DesktopBridgeRequest<TPayload>): Promise<TResult>;
}

declare global {
  interface Window {
    __NEXUS_DESKTOP_BRIDGE__?: NativeDesktopBridge;
  }
}

export function isDesktopBridgeAvailable(): boolean {
  return typeof window !== "undefined" && typeof window.__NEXUS_DESKTOP_BRIDGE__?.invoke === "function";
}

export async function getDesktopAppVersion(): Promise<DesktopAppVersion> {
  return invokeDesktopBridge<Record<string, never>, DesktopAppVersion>("app.get_app_version", {});
}

export async function exportDesktopLogs(): Promise<DesktopExportLogsResult> {
  return invokeDesktopBridge<Record<string, never>, DesktopExportLogsResult>("app.export_logs", {});
}

export async function openDesktopRoute(route: string): Promise<void> {
  await invokeDesktopBridge<{ route: string }, { opened: boolean }>("app.open_route", { route });
}

export async function getDesktopPersistentState(key: string): Promise<DesktopPersistentStateResult> {
  return invokeDesktopBridge<{ key: string }, DesktopPersistentStateResult>(
    "app.get_persistent_state",
    { key },
  );
}

export async function setDesktopPersistentState(key: string, value: string): Promise<void> {
  await invokeDesktopBridge<{ key: string; value: string }, { saved: boolean }>(
    "app.set_persistent_state",
    { key, value },
  );
}

export async function removeDesktopPersistentState(key: string): Promise<void> {
  await invokeDesktopBridge<{ key: string }, { removed: boolean }>(
    "app.remove_persistent_state",
    { key },
  );
}

async function invokeDesktopBridge<TPayload, TResult>(
  kind: DesktopBridgeKind,
  payload: TPayload,
): Promise<TResult> {
  const bridge = typeof window !== "undefined" ? window.__NEXUS_DESKTOP_BRIDGE__ : undefined;
  if (!bridge) {
    throw new Error("Desktop bridge is unavailable");
  }
  return bridge.invoke<TPayload, TResult>({
    schema_version: 1,
    kind,
    payload,
  });
}
