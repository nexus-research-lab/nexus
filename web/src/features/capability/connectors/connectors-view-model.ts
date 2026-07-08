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
  searchQuery: string;
  setSearchQuery: (q: string) => void;
  activeCategory: string;
  setActiveCategory: (c: string) => void;
  connectedCount: number;
  selectedDetail: ConnectorDetail | null;
  detailLoading: boolean;
  deviceAuthSession: ConnectorDeviceAuthStart | null;
  openDetail: (connectorId: string) => Promise<void>;
  closeDetail: () => void;
  closeDeviceAuthSession: () => void;
  handleConnect: (connectorId: string) => Promise<void>;
  handleConnectWithCredential: (connectorId: string, credential: string) => Promise<boolean>;
  handleDisconnect: (connectorId: string) => Promise<void>;
  handleSaveOauthClient: (connectorId: string, clientId: string, clientSecret: string) => Promise<boolean>;
  handleDeleteOauthClient: (connectorId: string) => Promise<boolean>;
  busyId: string | null;
  statusMessage: string | null;
  errorMessage: string | null;
  setStatusMessage: (m: string | null) => void;
  setErrorMessage: (m: string | null) => void;
  refresh: () => void;
}
