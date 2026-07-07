export interface RuntimeSettings {
  workspace_path?: string;
  current_workspace_path?: string;
  restart_required?: boolean;
  updated_at?: string;
}

export interface UpdateRuntimeSettingsParams {
  workspace_path?: string;
}
