"use client";

/**
 * 连接器控制器 Hook —— 管理连接器页面的搜索、过滤、连接/断开等状态。
 */

import { useCallback, useEffect, useMemo, useState } from "react";

import {
  connect_connector_api,
  delete_connector_oauth_client_api,
  disconnect_connector_api,
  get_connector_auth_url_api,
  get_connector_detail_api,
  get_connectors_api,
  save_connector_oauth_client_api,
  start_connector_device_auth_api,
} from "@/lib/api/connector-api";
import { get_connector_oauth_redirect_uri, is_desktop_runtime } from "@/config/desktop-runtime";
import { open_shop_prompt } from "@/features/capability/connectors/shop-domain-prompt";
import { ConnectorDetail, ConnectorDeviceAuthStart, ConnectorInfo } from "@/types/capability/connector";
import type { ConnectorDirectoryController } from "@/features/capability/connectors/connectors-view-model";

export function useConnectorController(): ConnectorDirectoryController {
  const [all_connectors, set_all_connectors] = useState<ConnectorInfo[]>([]);
  const [loading, set_loading] = useState(true);
  const [search_query, set_search_query] = useState("");
  const [active_category, set_active_category] = useState("all");
  const [selected_detail, set_selected_detail] = useState<ConnectorDetail | null>(null);
  const [detail_loading, set_detail_loading] = useState(false);
  const [device_auth_session, set_device_auth_session] = useState<ConnectorDeviceAuthStart | null>(null);
  const [busy_id, set_busy_id] = useState<string | null>(null);
  const [status_message, set_status_message] = useState<string | null>(null);
  const [error_message, set_error_message] = useState<string | null>(null);

  // 加载连接器列表
  const load = useCallback(async () => {
    set_loading(true);
    try {
      const items = await get_connectors_api();
      set_all_connectors(items);
    } catch (e) {
      set_error_message(e instanceof Error ? e.message : "加载失败");
    } finally {
      set_loading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  // 过滤后的连接器
  const connectors = useMemo(() => {
    let filtered = all_connectors;
    // 按类别过滤
    if (active_category !== "all") {
      filtered = filtered.filter((c) => c.category === active_category);
    }
    // 按搜索词过滤
    if (search_query.trim()) {
      const q = search_query.toLowerCase();
      filtered = filtered.filter(
        (c) =>
          c.title.toLowerCase().includes(q) ||
          c.name.toLowerCase().includes(q) ||
          c.description.toLowerCase().includes(q),
      );
    }
    return filtered;
  }, [all_connectors, active_category, search_query]);

  // 已连接数量
  const connected_count = useMemo(
    () => all_connectors.filter((c) => c.connection_state === "connected").length,
    [all_connectors],
  );

  // 打开详情
  const open_detail = useCallback(async (connector_id: string) => {
    set_detail_loading(true);
    set_selected_detail(null);
    try {
      const detail = await get_connector_detail_api(connector_id);
      set_selected_detail(detail);
    } catch (e) {
      set_error_message(e instanceof Error ? e.message : "获取详情失败");
    } finally {
      set_detail_loading(false);
    }
  }, []);

  const close_detail = useCallback(() => {
    set_selected_detail(null);
  }, []);

  const close_device_auth_session = useCallback(() => {
    set_device_auth_session(null);
  }, []);

  // 连接 —— OAuth 类型打开授权窗口，API Key 类型由专用弹窗保存凭证
  const handle_connect = useCallback(
    async (connector_id: string) => {
      set_busy_id(connector_id);
      try {
        // 查找该连接器信息，判断是否 OAuth
        const target = all_connectors.find((c) => c.connector_id === connector_id);
        if (target?.auth_type === "oauth2") {
          let shop: string | undefined;
          if (target.connector_id === "shopify" || target.requires_extra?.includes("shop")) {
            const prompted_shop = await open_shop_prompt();
            if (!prompted_shop) {
              return;
            }
            shop = prompted_shop;
          }

          if (target.connector_id === "github" && is_desktop_runtime()) {
            const session = await start_connector_device_auth_api(connector_id);
            set_device_auth_session(session);
            set_status_message("已生成 GitHub 授权码");
            const auth_url = session.verification_uri_complete || session.verification_uri;
            if (auth_url) {
              window.open(auth_url, "_blank", "noopener,noreferrer");
            }
            return;
          }

          // 获取 OAuth 授权 URL 并在新窗口打开
          const redirect_uri = get_connector_oauth_redirect_uri();
          const { auth_url } = await get_connector_auth_url_api(connector_id, redirect_uri, shop);
          if (!auth_url) {
            throw new Error("授权地址为空，请检查连接器配置");
          }
          const popup = window.open(
            auth_url,
            "_blank",
            "popup=yes,width=720,height=860",
          );
          if (!popup) {
            throw new Error("授权窗口被浏览器拦截，请允许弹窗后重试");
          }
          set_status_message("已打开授权页面，请在新窗口完成授权");
        } else if (target?.auth_type === "api_key" || target?.auth_type === "token") {
          set_error_message(target.auth_type === "token" ? "请填写 Token 后连接" : "请填写 API Key 后连接");
        } else {
          await connect_connector_api(connector_id);
          set_status_message("连接成功");
          await load();
          if (selected_detail?.connector_id === connector_id) {
            const detail = await get_connector_detail_api(connector_id);
            set_selected_detail(detail);
          }
        }
      } catch (e) {
        set_error_message(e instanceof Error ? e.message : "连接失败");
      } finally {
        set_busy_id(null);
      }
    },
    [load, selected_detail, all_connectors],
  );

  const handle_connect_with_api_key = useCallback(
    async (connector_id: string, api_key: string) => {
      set_busy_id(connector_id);
      try {
        const target = all_connectors.find((c) => c.connector_id === connector_id);
        const body = target?.auth_type === "token" ? { token: api_key } : { api_key };
        await connect_connector_api(connector_id, body);
        set_status_message("连接成功");
        await load();
        if (selected_detail?.connector_id === connector_id) {
          const detail = await get_connector_detail_api(connector_id);
          set_selected_detail(detail);
        }
        return true;
      } catch (e) {
        set_error_message(e instanceof Error ? e.message : "连接失败");
        return false;
      } finally {
        set_busy_id(null);
      }
    },
    [all_connectors, load, selected_detail],
  );

  // 断开
  const handle_disconnect = useCallback(
    async (connector_id: string) => {
      set_busy_id(connector_id);
      try {
        await disconnect_connector_api(connector_id);
        set_status_message("已断开连接");
        await load();
        if (selected_detail?.connector_id === connector_id) {
          const detail = await get_connector_detail_api(connector_id);
          set_selected_detail(detail);
        }
      } catch (e) {
        set_error_message(e instanceof Error ? e.message : "断开失败");
      } finally {
        set_busy_id(null);
      }
    },
    [load, selected_detail],
  );

  const handle_save_oauth_client = useCallback(
    async (connector_id: string, client_id: string, client_secret: string) => {
      set_busy_id(connector_id);
      try {
        await save_connector_oauth_client_api(connector_id, { client_id, client_secret });
        set_status_message("应用配置已保存");
        await load();
        const detail = await get_connector_detail_api(connector_id);
        set_selected_detail(detail);
        return true;
      } catch (e) {
        set_error_message(e instanceof Error ? e.message : "保存配置失败");
        return false;
      } finally {
        set_busy_id(null);
      }
    },
    [load],
  );

  const handle_delete_oauth_client = useCallback(
    async (connector_id: string) => {
      set_busy_id(connector_id);
      try {
        await delete_connector_oauth_client_api(connector_id);
        set_status_message("应用配置已删除");
        await load();
        const detail = await get_connector_detail_api(connector_id);
        set_selected_detail(detail);
        return true;
      } catch (e) {
        set_error_message(e instanceof Error ? e.message : "删除配置失败");
        return false;
      } finally {
        set_busy_id(null);
      }
    },
    [load],
  );

  return {
    connectors,
    loading,
    search_query,
    set_search_query,
    active_category,
    set_active_category,
    connected_count,
    selected_detail,
    detail_loading,
    open_detail,
    close_detail,
    device_auth_session,
    close_device_auth_session,
    handle_connect,
    handle_connect_with_api_key,
    handle_disconnect,
    handle_save_oauth_client,
    handle_delete_oauth_client,
    busy_id,
    status_message,
    error_message,
    set_status_message,
    set_error_message,
    refresh: load,
  };
}
