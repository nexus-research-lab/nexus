/**
 * =====================================================
 * @File   : connectors-view-model.ts
 * @Date   : 2026-04-16 13:35
 * @Author : leemysw
 * 2026-04-16 13:35   Create
 * =====================================================
 */

import type { ConnectorDetail, ConnectorInfo } from "@/types/capability/connector";

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
  open_detail: (connector_id: string) => void;
  close_detail: () => void;
  handle_connect: (connector_id: string) => Promise<void>;
  handle_disconnect: (connector_id: string) => Promise<void>;
  busy_id: string | null;
  status_message: string | null;
  error_message: string | null;
  set_status_message: (m: string | null) => void;
  set_error_message: (m: string | null) => void;
  refresh: () => void;
}
