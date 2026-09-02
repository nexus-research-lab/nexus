// INPUT: UiTabs 的当前值、切换命令与键盘操作。
// OUTPUT: 证明筛选/视图选择使用 button group，而不伪装成站点导航。
// POS: UiTabs DOM 语义测试；页面内容与路由切换由消费者负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it } from "vitest";

import { UiTabs } from "@/shared/ui/navigation/tabs";

describe("UiTabs", () => {
  it("uses a named button group and exposes the selected value", async () => {
    const user = userEvent.setup();

    function Harness() {
      const [value, setValue] = useState<"all" | "active">("all");
      return (
        <UiTabs
          activeValue={value}
          ariaLabel="状态筛选"
          onChange={setValue}
          options={[
            { label: "全部", value: "all" },
            { label: "活跃", value: "active" },
          ]}
        />
      );
    }

    render(<Harness />);
    expect(screen.getByRole("group", { name: "状态筛选" })).toBeTruthy();
    expect(screen.queryByRole("navigation")).toBeNull();

    const all = screen.getByRole("button", { name: "全部" });
    const active = screen.getByRole("button", { name: "活跃" });
    expect(all.getAttribute("aria-pressed")).toBe("true");
    expect(active.getAttribute("aria-pressed")).toBe("false");

    await user.click(active);
    expect(active.getAttribute("aria-pressed")).toBe("true");
    expect(active.hasAttribute("aria-current")).toBe(false);
  });

  it("keeps dismiss separate from selecting the active item", async () => {
    const user = userEvent.setup();
    const changes: string[] = [];
    const dismissals: string[] = [];
    render(
      <UiTabs
        activeValue="work"
        ariaLabel="工作区视图"
        dismissActiveLabel="关闭当前视图"
        onChange={(value) => changes.push(value)}
        onDismissActive={(value) => dismissals.push(value)}
        options={[
          { label: "工作", value: "work" },
          { label: "文件", value: "files" },
        ]}
      />,
    );

    await user.click(screen.getByRole("button", { name: "关闭当前视图" }));
    expect(dismissals).toEqual(["work"]);
    expect(changes).toEqual([]);
  });
});
