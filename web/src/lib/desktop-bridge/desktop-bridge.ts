export type DesktopBridgeKind =
  | "app.get_app_version"
  | "app.open_external_url"
  | "app.export_logs"
  | "app.open_route"
  | "app.close_launcher"
  | "app.get_global_shortcut_status"
  | "app.set_global_shortcut_enabled"
  | "app.set_global_shortcut_accelerator"
  | "app.reset_global_shortcut_accelerator";

export interface DesktopBridgeRequest<TPayload = Record<string, unknown>> {
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

export interface DesktopGlobalShortcutStatus {
  accelerator: string;
  default_accelerator: string;
  enabled: boolean;
  is_default: boolean;
  registered: boolean;
  error_message?: string;
}

interface NativeDesktopBridge {
  invoke<TPayload, TResult>(message: DesktopBridgeRequest<TPayload>): Promise<TResult>;
}

declare global {
  interface Window {
    __NEXUS_DESKTOP_BRIDGE__?: NativeDesktopBridge;
  }
}

export function is_desktop_bridge_available(): boolean {
  return typeof window !== "undefined" && typeof window.__NEXUS_DESKTOP_BRIDGE__?.invoke === "function";
}

export async function get_desktop_app_version(): Promise<DesktopAppVersion> {
  return invoke_desktop_bridge<Record<string, never>, DesktopAppVersion>("app.get_app_version", {});
}

export async function open_external_url(url: string): Promise<void> {
  await invoke_desktop_bridge<{ url: string }, { opened: boolean }>("app.open_external_url", { url });
}

export async function export_desktop_logs(): Promise<DesktopExportLogsResult> {
  return invoke_desktop_bridge<Record<string, never>, DesktopExportLogsResult>("app.export_logs", {});
}

export async function open_desktop_route(route: string): Promise<void> {
  await invoke_desktop_bridge<{ route: string }, { opened: boolean }>("app.open_route", { route });
}

export async function close_desktop_launcher(): Promise<void> {
  await invoke_desktop_bridge<Record<string, never>, { closed: boolean }>("app.close_launcher", {});
}

export async function get_desktop_global_shortcut_status(): Promise<DesktopGlobalShortcutStatus> {
  return invoke_desktop_bridge<Record<string, never>, DesktopGlobalShortcutStatus>(
    "app.get_global_shortcut_status",
    {},
  );
}

export async function set_desktop_global_shortcut_enabled(
  enabled: boolean,
): Promise<DesktopGlobalShortcutStatus> {
  return invoke_desktop_bridge<{ enabled: boolean }, DesktopGlobalShortcutStatus>(
    "app.set_global_shortcut_enabled",
    { enabled },
  );
}

export async function set_desktop_global_shortcut_accelerator(
  accelerator: string,
): Promise<DesktopGlobalShortcutStatus> {
  return invoke_desktop_bridge<{ accelerator: string }, DesktopGlobalShortcutStatus>(
    "app.set_global_shortcut_accelerator",
    { accelerator },
  );
}

export async function reset_desktop_global_shortcut_accelerator(): Promise<DesktopGlobalShortcutStatus> {
  return invoke_desktop_bridge<Record<string, never>, DesktopGlobalShortcutStatus>(
    "app.reset_global_shortcut_accelerator",
    {},
  );
}

async function invoke_desktop_bridge<TPayload, TResult>(
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
