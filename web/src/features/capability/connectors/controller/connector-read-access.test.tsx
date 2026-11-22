// INPUT: A successful catalog/detail followed by access revocation, read-only retry or a locale change.
// OUTPUT: Revoked snapshots are discarded and cannot reappear while retrying; reads follow the active locale.
// POS: Connector read access boundary regression.
import { act, renderHook, waitFor } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { ApiRequestError } from "@/lib/api/core/http-error";
import type { Locale } from "@/shared/i18n/locale-settings";
import { useConnectorCatalog } from "./use-connector-catalog";
import { useConnectorDetail } from "./use-connector-detail";
const api = vi.hoisted(() => ({ catalog: vi.fn(), detail: vi.fn() }));
vi.mock("@/lib/api/capability/connector-api", () => ({ getConnectorsApi: api.catalog, getConnectorDetailApi: api.detail }));
it("discards a denied catalog and detail until a successful authorized read", async () => {
  const item = { connector_id: "github", kind: "connector", status: "available", category: "developer", title: "GitHub", description: "Repository" };
  api.catalog.mockResolvedValueOnce([item]).mockRejectedValueOnce(new ApiRequestError("private", 403));
  api.detail.mockResolvedValueOnce(item).mockRejectedValueOnce(new ApiRequestError("private", 403));
  const catalog = renderHook(() => useConnectorCatalog({ failureFallback: "Failed", locale: "en" }));
  const detail = renderHook(() => useConnectorDetail({ failureFallback: "Failed", locale: "en" }));
  await waitFor(() => expect(catalog.result.current.loading).toBe(false));
  await act(async () => { await detail.result.current.openDetail("github"); });
  expect(catalog.result.current.allConnectors).toHaveLength(1);
  await act(async () => { await catalog.result.current.refresh(); await detail.result.current.refreshDetail("github"); });
  expect(catalog.result.current.allConnectors).toEqual([]);
  expect(detail.result.current.selectedDetail).toBeNull();
  api.catalog.mockReturnValue(new Promise(() => {}));
  api.detail.mockReturnValue(new Promise(() => {}));
  act(() => { void catalog.result.current.refresh(); void detail.result.current.refreshDetail("github"); });
  expect(catalog.result.current.allConnectors).toEqual([]);
  expect(detail.result.current.selectedDetail).toBeNull();
});
it("reads the catalog and an open detail in the active locale and rereads them when it changes", async () => {
  const item = { connector_id: "github", kind: "connector", status: "available", category: "developer", title: "GitHub", description: "Repository" };
  api.catalog.mockReset().mockResolvedValue([item]);
  api.detail.mockReset().mockResolvedValue(item);
  const catalog = renderHook(({ locale }: { locale: Locale }) => useConnectorCatalog({ failureFallback: "Failed", locale }), { initialProps: { locale: "zh" } });
  const detail = renderHook(({ locale }: { locale: Locale }) => useConnectorDetail({ failureFallback: "Failed", locale }), { initialProps: { locale: "zh" } });
  await waitFor(() => expect(catalog.result.current.loading).toBe(false));
  await act(async () => { await detail.result.current.openDetail("github"); });
  expect(api.catalog).toHaveBeenLastCalledWith({ locale: "zh", status: "available" });
  expect(api.detail).toHaveBeenLastCalledWith("github", "zh");
  catalog.rerender({ locale: "en" });
  detail.rerender({ locale: "en" });
  await waitFor(() => expect(api.catalog).toHaveBeenLastCalledWith({ locale: "en", status: "available" }));
  await waitFor(() => expect(api.detail).toHaveBeenLastCalledWith("github", "en"));
  expect(api.detail).toHaveBeenCalledTimes(2);
});
