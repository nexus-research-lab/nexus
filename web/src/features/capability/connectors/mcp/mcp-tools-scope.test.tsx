// INPUT: 固定和自定义 MCP 读取控制器、受控 API 完成顺序与配置切换。
// OUTPUT: 同配置刷新保留快照，跨配置隐藏快照/错误，迟到响应不得覆盖当前结果。
// POS: 工具目录读取身份边界回归；不连接外部服务。
import { useMemo } from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { getConnectorMCPToolsApi, getCustomMCPToolsApi } from "@/lib/api/capability/connector-api";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import type { ConnectorDetail, CustomMCPServer, CustomMCPToolCatalog } from "@/types/capability/connector";

import { useCustomMCPTools } from "../custom/detail/use-custom-mcp-tools";
import { useConnectorMCPTools } from "../detail/use-connector-mcp-tools";

vi.mock("@/lib/api/capability/connector-api", () => ({
  getConnectorMCPToolsApi: vi.fn(), getCustomMCPToolsApi: vi.fn(),
}));

const custom: CustomMCPServer = {
  connector_id: "custom-mcp:one", name: "one", enabled: true,
  configuration_state: "ready", type: "http", url: "https://one.test/mcp",
};
const fixed: ConnectorDetail = {
  connector_id: "richmail", name: "richmail", title: "RichMail", description: "Mail",
  auth_type: "local_pairing", category: "productivity", connection_state: "connected",
  icon: "richmail", is_configured: true, kind: "connector", status: "available",
  scopes: [], features: [], mcp_server_url: "https://one.test/mcp",
};
const catalog: CustomMCPToolCatalog = {
  inspection_state: "connected", supports_tools: true,
  tools: [{ name: "read", title: "Read", arguments: [] }],
};

function deferred() {
  let resolve!: (value: CustomMCPToolCatalog) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<CustomMCPToolCatalog>((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}

beforeEach(() => vi.clearAllMocks());

describe.each([
  { name: "custom MCP", api: getCustomMCPToolsApi,
    useTools: (url: string) => useCustomMCPTools(useMemo(() => ({ ...custom, url }), [url])) },
  { name: "fixed connector", api: getConnectorMCPToolsApi,
    useTools: (url: string) => useConnectorMCPTools(useMemo(() => ({ ...fixed, mcp_server_url: url }), [url])) },
])("$name tool scope", ({ api, useTools }) => {
  it("retains same-scope refresh data, hides it on scope change and ignores late completion", async () => {
    const first = deferred(); const retry = deferred(); const next = deferred();
    vi.mocked(api).mockReturnValueOnce(first.promise).mockReturnValueOnce(retry.promise).mockReturnValueOnce(next.promise);
    const view = renderHook(({ url }) => useTools(url), {
      initialProps: { url: "https://one.test/mcp" }, wrapper: I18nProvider,
    });
    await act(async () => first.resolve(catalog));
    expect(view.result.current.catalog).toEqual(catalog);
    act(() => view.result.current.refresh());
    expect(view.result.current.catalog).toEqual(catalog);
    view.rerender({ url: "https://two.test/mcp" });
    expect(view.result.current.catalog).toBeNull();
    await act(async () => retry.resolve(catalog));
    expect(view.result.current.catalog).toBeNull();
    const newCatalog = { ...catalog, tools: [] };
    await act(async () => next.resolve(newCatalog));
    await waitFor(() => expect(view.result.current.catalog).toEqual(newCatalog));
  });

  it("does not expose a previous configuration failure while reading the next one", async () => {
    const first = deferred(); const next = deferred();
    vi.mocked(api).mockReturnValueOnce(first.promise).mockReturnValueOnce(next.promise);
    const view = renderHook(({ url }) => useTools(url), {
      initialProps: { url: "https://one.test/mcp" }, wrapper: I18nProvider,
    });
    await act(async () => first.reject(new Error("offline")));
    expect(view.result.current.failure).not.toBeNull();
    view.rerender({ url: "https://two.test/mcp" });
    expect(view.result.current.failure).toBeNull();
    await act(async () => next.resolve(catalog));
    expect(view.result.current.catalog).toEqual(catalog);
  });
});
