// INPUT: Uncertain Connector writes followed by successful reads.
// OUTPUT: Reads never replay or unlock an unproven write; explicit new intent does.
// POS: Connector and custom MCP mutation recovery regression.
import { act, renderHook } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { useConnectorCommands } from "./use-connector-commands";
import { useConnectorCommand } from "./use-connector-command";
import { useCustomMCPServers } from "../custom/use-custom-mcp-servers";
import type { ConnectorFeedback } from "./connector-controller-types";
import type { CustomMCPServer } from "@/types/capability/connector";
const api = vi.hoisted(() => ({ disconnect: vi.fn(), list: vi.fn(), toggle: vi.fn() }));
vi.mock("@/lib/api/capability/connector-api", () => ({ disconnectConnectorApi: api.disconnect, getCustomMCPServersApi: api.list, setCustomMCPServerEnabledApi: api.toggle }));
vi.mock("@/shared/i18n/i18n-context", () => ({ useI18n: () => ({ t: translate }) }));
function translate(key: string) { return key; }
beforeEach(() => { vi.clearAllMocks(); });
it("retains fixed connector unknown lock after GET, then accepts explicit new intent", async () => {
  api.disconnect.mockRejectedValue(new Error("uncertain"));
  let feedback: ConnectorFeedback;
  const reportFeedback = (value: ConnectorFeedback) => { feedback = value; };
  const refresh = vi.fn().mockResolvedValue(true);
  const { result } = renderHook(() => {
    const gate = useConnectorCommand();
    return { gate, commands: useConnectorCommands({ ...gate, connectors: [], refreshCatalog: vi.fn(), refreshConnector: refresh, reportFeedback, requestShopDomain: vi.fn() }) };
  });
  await act(async () => { await result.current.commands.handleDisconnect("github"); });
  await act(async () => { feedback.action?.onClick(); });
  expect(refresh).toHaveBeenCalledOnce();
  expect(result.current.gate.reconciliationActions).toHaveLength(1);
  await act(async () => { await result.current.commands.handleDisconnect("github"); });
  expect(api.disconnect).toHaveBeenCalledOnce();
  act(() => { feedback.action?.onClick(); });
  expect(result.current.gate.reconciliationActions).toHaveLength(0);
});
it("custom MCP unknown lock survives read and blocks repeated writes", async () => {
  api.toggle.mockRejectedValue(new Error("uncertain"));
  api.list.mockResolvedValue([]);
  let feedback: ConnectorFeedback;
  const reportFeedback = (value: ConnectorFeedback) => { feedback = value; };
  const { result } = renderHook(() => useCustomMCPServers({ enabled: false, onCatalogChanged: vi.fn(), reportFeedback }));
  const server = { connector_id: "custom:test", name: "Test" } as CustomMCPServer;
  await act(async () => { await result.current.setEnabled(server, true); });
  expect(result.current.blocked).toBe(true);
  expect(result.current.busy).toBe(false);
  await act(async () => { feedback.action?.onClick(); });
  expect(result.current.blocked).toBe(true);
  await act(async () => { await result.current.setEnabled(server, true); });
  expect(api.toggle).toHaveBeenCalledOnce();
  act(() => { feedback.action?.onClick(); });
  expect(result.current.blocked).toBe(false);
});
