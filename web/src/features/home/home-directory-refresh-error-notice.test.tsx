// INPUT: Home stale 目录刷新失败事实和显式重读命令。
// OUTPUT: 证明 Launcher/侧栏共用提示通过 UiInlineNotice 展示，并只委托一次安全重读。
// POS: Home 目录降级提示 DOM 合同；目录快照保留和请求栅栏归 home-directory-store。

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { HomeDirectoryRefreshErrorNotice } from "./home-directory-refresh-error-notice";

describe("HomeDirectoryRefreshErrorNotice", () => {
  it("uses the shared danger notice and delegates a single safe read retry", () => {
    const onRetry = vi.fn();
    render(
      <I18N_CONTEXT.Provider
        value={{
          locale: "zh",
          setLocale: vi.fn(),
          t: (key) => key,
        }}
      >
        <HomeDirectoryRefreshErrorNotice onRetry={onRetry} />
      </I18N_CONTEXT.Provider>,
    );

    const status = screen.getByRole("status");
    expect(status.getAttribute("data-inline-notice-tone")).toBe("danger");
    expect(screen.getByText("sidebar.directory_refresh_failed_description")).toBeTruthy();
    expect(screen.getByText("sidebar.directory_refresh_failed_impact")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "sidebar.retry" }));
    expect(onRetry).toHaveBeenCalledOnce();
  });
});
