/**
 * =====================================================
 * @File   : connectors-view-model.ts
 * @Date   : 2026-04-16 13:35
 * @Author : leemysw
 * 2026-04-16 13:35   Create
 * =====================================================
 */

import type { ConnectorDetail, ConnectorDeviceAuthStart, ConnectorInfo } from "@/types/capability/connector";

export interface ConnectorDirectoryController {
  connectors: ConnectorInfo[];
  loading: boolean;
  search_query: string;
  set_search_query: (q: string) => void;
  active_category: string;
  set_active_category: (c: string) => void;
  connected_count: number;
  selected_detail: ConnectorDetail | null;
  detail_loading: boolean;
  device_auth_session: ConnectorDeviceAuthStart | null;
  open_detail: (connector_id: string) => void;
  close_detail: () => void;
  close_device_auth_session: () => void;
  handle_connect: (connector_id: string) => Promise<void>;
  handle_disconnect: (connector_id: string) => Promise<void>;
  handle_save_oauth_client: (connector_id: string, client_id: string, client_secret: string) => Promise<boolean>;
  handle_delete_oauth_client: (connector_id: string) => Promise<boolean>;
  busy_id: string | null;
  status_message: string | null;
  error_message: string | null;
  set_status_message: (m: string | null) => void;
  set_error_message: (m: string | null) => void;
  refresh: () => void;
}
