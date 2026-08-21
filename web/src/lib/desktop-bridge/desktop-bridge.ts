type DesktopBridgeKind =
  | "app.get_app_version"
  | "app.get_state_root"
  | "app.choose_state_root"
  | "app.relocate_state_root"
  | "app.open_external_url"
  | "app.start_browser_extension_setup"
  | "app.get_workspace_file_applications"
  | "app.open_workspace_file"
  | "app.export_logs"
  | "app.open_route"
  | "app.start_update"
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

export interface DesktopFileApplication {
  name: string;
  path: string;
}

export interface DesktopFileApplicationsResult {
  applications: DesktopFileApplication[];
  default_application?: DesktopFileApplication | null;
}

export type DesktopWorkspaceFileOpenTarget =
  | "default"
  | "file_manager"
  | "terminal"
  | "application";

export interface DesktopStateRootStatus {
  current_path: string;
  default_path: string;
  migration_error?: string | null;
}

export interface DesktopStateRootMigrationResult {
  restarting: boolean;
  target_path: string;
}

export interface DesktopStateRootSelectionResult {
  cancelled: boolean;
  path?: string;
}

export interface DesktopPersistentStateResult {
  key: string;
  value?: string | null;
}

export interface DesktopUpdateStartResult {
  status: "disabled" | "in_progress" | "started" | "unavailable";
}

export interface DesktopBrowserExtensionSetupResult {
  browser: "chrome" | "edge";
  browser_name: string;
  opened: boolean;
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

export async function getDesktopStateRoot(): Promise<DesktopStateRootStatus> {
  return invokeDesktopBridge<Record<string, never>, DesktopStateRootStatus>("app.get_state_root", {});
}

export async function chooseDesktopStateRoot(
  initialPath: string,
  title: string,
  prompt: string,
): Promise<DesktopStateRootSelectionResult> {
  return chooseDesktopDirectory(initialPath, title, prompt);
}

export async function chooseDesktopDirectory(
  initialPath: string,
  title: string,
  prompt: string,
): Promise<DesktopStateRootSelectionResult> {
  return invokeDesktopBridge<
    { initial_path: string; prompt: string; title: string },
    DesktopStateRootSelectionResult
  >(
    "app.choose_state_root",
    { initial_path: initialPath, prompt, title },
  );
}

export async function relocateDesktopStateRoot(path: string): Promise<DesktopStateRootMigrationResult> {
  return invokeDesktopBridge<{ path: string }, DesktopStateRootMigrationResult>(
    "app.relocate_state_root",
    { path },
  );
}

export async function openDesktopExternalURL(url: string): Promise<void> {
  await invokeDesktopBridge<{ url: string }, { opened: boolean }>(
    "app.open_external_url",
    { url },
  );
}

export async function startDesktopBrowserExtensionSetup(): Promise<DesktopBrowserExtensionSetupResult> {
  return invokeDesktopBridge<Record<string, never>, DesktopBrowserExtensionSetupResult>(
    "app.start_browser_extension_setup",
    {},
  );
}

export async function getDesktopWorkspaceFileApplications(
  path: string,
): Promise<DesktopFileApplicationsResult> {
  return invokeDesktopBridge<
    { path: string },
    DesktopFileApplicationsResult
  >("app.get_workspace_file_applications", { path });
}

export async function openDesktopWorkspaceFile(
  path: string,
  target: DesktopWorkspaceFileOpenTarget,
  applicationPath?: string,
): Promise<void> {
  await invokeDesktopBridge<
    { application_path: string; path: string; target: DesktopWorkspaceFileOpenTarget },
    { opened: boolean }
  >("app.open_workspace_file", {
    application_path: applicationPath ?? "",
    path,
    target,
  });
}

export async function exportDesktopLogs(): Promise<DesktopExportLogsResult> {
  return invokeDesktopBridge<Record<string, never>, DesktopExportLogsResult>("app.export_logs", {});
}

export async function openDesktopRoute(route: string): Promise<void> {
  await invokeDesktopBridge<{ route: string }, { opened: boolean }>("app.open_route", { route });
}

export async function startDesktopUpdate(): Promise<DesktopUpdateStartResult> {
  return invokeDesktopBridge<Record<string, never>, DesktopUpdateStartResult>(
    "app.start_update",
    {},
  );
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
