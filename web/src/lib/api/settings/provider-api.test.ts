import { beforeEach, describe, expect, it, vi } from "vitest";

const { requestApi } = vi.hoisted(() => ({ requestApi: vi.fn() }));

vi.mock("@/lib/api/core/http", () => ({ requestApi }));

import { listProviderOptionsApi } from "./provider-api";

describe("listProviderOptionsApi", () => {
  beforeEach(() => requestApi.mockReset());

  it("shares one in-flight request per runtime and re-requests after settling", async () => {
    let resolve: (value: unknown) => void = () => {};
    requestApi.mockReturnValueOnce(new Promise((next) => {
      resolve = next;
    }));
    requestApi.mockResolvedValueOnce({ items: [] });
    requestApi.mockResolvedValueOnce({ items: ["second"] });

    const first = listProviderOptionsApi("nxs");
    const second = listProviderOptionsApi(" nxs ");
    const other = listProviderOptionsApi("claude");
    expect(second).toBe(first);
    expect(requestApi).toHaveBeenCalledTimes(2);

    resolve({ items: ["first"] });
    await expect(first).resolves.toEqual({ items: ["first"] });
    await other;

    // 结算后不缓存：下一次读取重新请求。
    await expect(listProviderOptionsApi("nxs")).resolves.toEqual({ items: ["second"] });
    expect(requestApi).toHaveBeenCalledTimes(3);
  });
});
