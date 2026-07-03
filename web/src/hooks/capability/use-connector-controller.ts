"use client";

/**
 * 连接器控制器 Hook —— 管理连接器页面的搜索、过滤、连接/断开等状态。
 */

import { useCallback, useEffect, useMemo, useState } from "react";

import {
  connectConnectorApi,
  deleteConnectorOauthClientApi,
  disconnectConnectorApi,
  getConnectorAuthUrlApi,
  getConnectorDetailApi,
  getConnectorsApi,
  saveConnectorOauthClientApi,
  startConnectorDeviceAuthApi,
} from "@/lib/api/connector-api";
import { getConnectorOauthRedirectUri, isDesktopRuntime } from "@/config/desktop-runtime";
import {
  buildDirectCredentialPayload,
  getDirectCredentialLabel,
  isDirectCredentialAuth,
} from "@/features/capability/connectors/connector-auth";
import { openShopPrompt } from "@/features/capability/connectors/shop-domain-prompt";
import { ConnectorDetail, ConnectorDeviceAuthStart, ConnectorInfo } from "@/types/capability/connector";
import type { ConnectorDirectoryController } from "@/features/capability/connectors/connectors-view-model";

export function useConnectorController(): ConnectorDirectoryController {
  const [allConnectors, setAllConnectors] = useState<ConnectorInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [activeCategory, setActiveCategory] = useState("all");
  const [selectedDetail, setSelectedDetail] = useState<ConnectorDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [deviceAuthSession, setDeviceAuthSession] = useState<ConnectorDeviceAuthStart | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [statusMessage, setStatusMessage] = useState<string | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  // 加载连接器列表
  const load = useCallback(async () => {
    setLoading(true);
    try {
      const items = await getConnectorsApi();
      setAllConnectors(items);
    } catch (e) {
      setErrorMessage(e instanceof Error ? e.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  // 过滤后的连接器
  const connectors = useMemo(() => {
    let filtered = allConnectors;
    // 按类别过滤
    if (activeCategory !== "all") {
      filtered = filtered.filter((c) => c.category === activeCategory);
    }
    // 按搜索词过滤
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase();
      filtered = filtered.filter(
        (c) =>
          c.title.toLowerCase().includes(q) ||
          c.name.toLowerCase().includes(q) ||
          c.description.toLowerCase().includes(q),
      );
    }
    return filtered;
  }, [allConnectors, activeCategory, searchQuery]);

  // 已连接数量
  const connectedCount = useMemo(
    () => allConnectors.filter((c) => c.connection_state === "connected").length,
    [allConnectors],
  );

  // 打开详情
  const openDetail = useCallback(async (connectorId: string) => {
    setDetailLoading(true);
    setSelectedDetail(null);
    try {
      const detail = await getConnectorDetailApi(connectorId);
      setSelectedDetail(detail);
    } catch (e) {
      setErrorMessage(e instanceof Error ? e.message : "获取详情失败");
    } finally {
      setDetailLoading(false);
    }
  }, []);

  const closeDetail = useCallback(() => {
    setSelectedDetail(null);
  }, []);

  const closeDeviceAuthSession = useCallback(() => {
    setDeviceAuthSession(null);
  }, []);

  // 连接 —— OAuth 类型打开授权窗口，直接凭证类型由专用弹窗保存。
  const handleConnect = useCallback(
    async (connectorId: string) => {
      setBusyId(connectorId);
      try {
        // 查找该连接器信息，判断是否 OAuth
        const target = allConnectors.find((c) => c.connector_id === connectorId);
        if (target?.auth_type === "oauth2") {
          let shop: string | undefined;
          if (target.connector_id === "shopify" || target.requires_extra?.includes("shop")) {
            const promptedShop = await openShopPrompt();
            if (!promptedShop) {
              return;
            }
            shop = promptedShop;
          }

          if (target.connector_id === "github" && isDesktopRuntime()) {
            const session = await startConnectorDeviceAuthApi(connectorId);
            setDeviceAuthSession(session);
            setStatusMessage("已生成 GitHub 授权码");
            const authUrl = session.verification_uri_complete || session.verification_uri;
            if (authUrl) {
              window.open(authUrl, "_blank", "noopener,noreferrer");
            }
            return;
          }

          // 获取 OAuth 授权 URL 并在新窗口打开
          const redirectUri = getConnectorOauthRedirectUri();
          const { auth_url: authUrl } = await getConnectorAuthUrlApi(connectorId, redirectUri, shop);
          if (!authUrl) {
            throw new Error("授权地址为空，请检查连接器配置");
          }
          const popup = window.open(
            authUrl,
            "_blank",
            "popup=yes,width=720,height=860",
          );
          if (!popup) {
            throw new Error("授权窗口被浏览器拦截，请允许弹窗后重试");
          }
          setStatusMessage("已打开授权页面，请在新窗口完成授权");
        } else if (isDirectCredentialAuth(target?.auth_type)) {
          setErrorMessage(`请填写 ${getDirectCredentialLabel(target?.auth_type)} 后连接`);
        } else {
          await connectConnectorApi(connectorId);
          setStatusMessage("连接成功");
          await load();
          if (selectedDetail?.connector_id === connectorId) {
            const detail = await getConnectorDetailApi(connectorId);
            setSelectedDetail(detail);
          }
        }
      } catch (e) {
        setErrorMessage(e instanceof Error ? e.message : "连接失败");
      } finally {
        setBusyId(null);
      }
    },
    [load, selectedDetail, allConnectors],
  );

  const handleConnectWithCredential = useCallback(
    async (connectorId: string, credential: string) => {
      setBusyId(connectorId);
      try {
        const target = allConnectors.find((c) => c.connector_id === connectorId);
        if (!target || !isDirectCredentialAuth(target.auth_type)) {
          throw new Error("当前连接器不支持直接凭证连接");
        }
        await connectConnectorApi(connectorId, buildDirectCredentialPayload(target.auth_type, credential));
        setStatusMessage("连接成功");
        await load();
        if (selectedDetail?.connector_id === connectorId) {
          const detail = await getConnectorDetailApi(connectorId);
          setSelectedDetail(detail);
        }
        return true;
      } catch (e) {
        setErrorMessage(e instanceof Error ? e.message : "连接失败");
        return false;
      } finally {
        setBusyId(null);
      }
    },
    [allConnectors, load, selectedDetail],
  );

  // 断开
  const handleDisconnect = useCallback(
    async (connectorId: string) => {
      setBusyId(connectorId);
      try {
        await disconnectConnectorApi(connectorId);
        setStatusMessage("已断开连接");
        await load();
        if (selectedDetail?.connector_id === connectorId) {
          const detail = await getConnectorDetailApi(connectorId);
          setSelectedDetail(detail);
        }
      } catch (e) {
        setErrorMessage(e instanceof Error ? e.message : "断开失败");
      } finally {
        setBusyId(null);
      }
    },
    [load, selectedDetail],
  );

  const handleSaveOauthClient = useCallback(
    async (connectorId: string, clientId: string, clientSecret: string) => {
      setBusyId(connectorId);
      try {
        await saveConnectorOauthClientApi(connectorId, { client_id: clientId, client_secret: clientSecret });
        setStatusMessage("应用配置已保存");
        await load();
        const detail = await getConnectorDetailApi(connectorId);
        setSelectedDetail(detail);
        return true;
      } catch (e) {
        setErrorMessage(e instanceof Error ? e.message : "保存配置失败");
        return false;
      } finally {
        setBusyId(null);
      }
    },
    [load],
  );

  const handleDeleteOauthClient = useCallback(
    async (connectorId: string) => {
      setBusyId(connectorId);
      try {
        await deleteConnectorOauthClientApi(connectorId);
        setStatusMessage("应用配置已删除");
        await load();
        const detail = await getConnectorDetailApi(connectorId);
        setSelectedDetail(detail);
        return true;
      } catch (e) {
        setErrorMessage(e instanceof Error ? e.message : "删除配置失败");
        return false;
      } finally {
        setBusyId(null);
      }
    },
    [load],
  );

  return {
    connectors,
    loading,
    searchQuery,
    setSearchQuery,
    activeCategory,
    setActiveCategory,
    connectedCount,
    selectedDetail,
    detailLoading,
    openDetail,
    closeDetail,
    deviceAuthSession,
    closeDeviceAuthSession,
    handleConnect,
    handleConnectWithCredential,
    handleDisconnect,
    handleSaveOauthClient,
    handleDeleteOauthClient,
    busyId,
    statusMessage,
    errorMessage,
    setStatusMessage,
    setErrorMessage,
    refresh: load,
  };
}
