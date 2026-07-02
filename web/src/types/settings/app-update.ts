export type AppUpdateStatusCode =
  | "disabled"
  | "failed"
  | "update_available"
  | "up_to_date";

export interface AppUpdateStatus {
  status: AppUpdateStatusCode;
  current_version: string;
  current_build_number: string;
  latest_version?: string | null;
  latest_build_number?: string | null;
  release_page_url?: string | null;
  can_download_installer: boolean;
  error_message?: string | null;
}

export interface AppUpdateDownloadResult {
  installer_path: string;
  sha256_path: string;
  sha256_hash: string;
}
