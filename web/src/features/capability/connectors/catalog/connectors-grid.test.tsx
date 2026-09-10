// INPUT: Connector 目录的读取中、读取失败和成功空集。
// OUTPUT: 统一资源状态保留读取优先级和显式恢复动作，不自动重放读取命令。
// POS: 目录状态组合回归；资源请求生命周期由目录控制器负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { expect, it, vi } from "vitest";

import { getResourceFailure } from "@/lib/error-message";
import { I18nProvider } from "@/shared/i18n/i18n-provider";

import { ConnectorsGrid } from "./connectors-grid";

it("keeps loading, failure recovery and a successful empty catalog distinct", async () => {
  const user = userEvent.setup();
  const props: ComponentProps<typeof ConnectorsGrid> = {
    activeCategory: "all", connectors: [], failure: getResourceFailure(new Error("offline"), "Catalog unavailable"),
    loading: true, onConnect: vi.fn(), onDisconnect: vi.fn(), onOpenConnector: vi.fn(), onRefresh: vi.fn(),
    pendingAction: null, reconciliationActions: [], searchQuery: "",
  };
  const { rerender } = render(<ConnectorsGrid {...props} />, { wrapper: I18nProvider });
  expect(screen.getByRole("status").getAttribute("data-resource-state")).toBe("loading");
  expect(screen.getByRole("status").getAttribute("aria-busy")).toBe("true");
  expect(screen.queryByRole("button")).toBeNull();
  rerender(<ConnectorsGrid {...props} loading={false} />);
  expect(screen.getByRole("status").getAttribute("data-resource-state")).toBe("error");
  expect(props.onRefresh).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button"));
  expect(props.onRefresh).toHaveBeenCalledOnce();
  rerender(<ConnectorsGrid {...props} loading={false} failure={null} />);
  expect(screen.getByRole("status").getAttribute("data-resource-state")).toBe("empty");
  expect(screen.queryByRole("button")).toBeNull();
  expect(props.onConnect).not.toHaveBeenCalled();
  expect(props.onDisconnect).not.toHaveBeenCalled();
});
