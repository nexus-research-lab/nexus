export type AppUpdateStatusKind =
  | "unsupported"
  | "unavailable"
  | "up_to_date"
  | "update_available"
  | "downloaded"
  | "error";

export interface AppUpdateReleaseInfo {
  version?: string;
  build_number?: string;
  release_name?: string | null;
  release_page_url?: string | null;
  published_at?: string | null;
  is_prerelease?: boolean;
  source?: string | null;
  asset_name?: string | null;
  asset_url?: string | null;
  sha256?: string | null;
  size?: number | null;
}

export interface AppUpdateStatus {
  status: AppUpdateStatusKind;
  current_version?: string | null;
  current_build_number?: string | null;
  latest?: AppUpdateReleaseInfo | null;
  message?: string | null;
  checked_at?: string | null;
  can_download?: boolean;
}

export interface AppUpdateDownloadResult {
  status: AppUpdateStatusKind;
  path?: string | null;
  file_name?: string | null;
  sha256?: string | null;
  size?: number | null;
  release?: AppUpdateReleaseInfo | null;
  message?: string | null;
}
