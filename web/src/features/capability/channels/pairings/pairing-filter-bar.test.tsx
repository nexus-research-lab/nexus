// INPUT: 配对状态计数、当前筛选与筛选更新命令。
// OUTPUT: 证明配对状态复用能力目录下划线标签，并发送精确状态值。
// POS: 配对筛选条 DOM 合同；查询、渠道和 Agent 选项行为由共享 Form/Menu 覆盖。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { PairingFilterBar } from "./pairing-filter-bar";

describe("PairingFilterBar", () => {
  it("uses the shared underline tabs and dispatches the selected status", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <I18N_CONTEXT.Provider
        value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
      >
        <PairingFilterBar
          agents={[]}
          counts={{ active: 3, all: 7, inactive: 1, pending: 2 }}
          filters={{ agentId: "", channel: "", query: "", status: "pending" }}
          onChange={onChange}
          searchPlaceholder="搜索配对"
        />
      </I18N_CONTEXT.Provider>,
    );

    const tabs = screen.getByRole("group", { name: "按配对状态筛选" });
    const pending = screen.getByRole("button", { name: /待处理/ });
    expect(tabs.className).toContain("w-fit");
    expect(tabs.className).not.toContain("segmented-control");
    expect(pending.getAttribute("aria-pressed")).toBe("true");
    expect(pending.className).toContain("border-(--text-strong)");
    expect(pending.className).not.toContain("radius-control-sm");

    await user.click(screen.getByRole("button", { name: /已授权/ }));
    expect(onChange).toHaveBeenCalledWith("status", "active");
  });
});
