// INPUT: UiTabs/UiDirectoryTabs 的当前值、切换命令与键盘操作。
// OUTPUT: 证明筛选/视图选择使用 button group，并提供跨领域目录紧凑预设。
// POS: 导航选择 DOM 语义测试；页面内容、路由和业务筛选状态由消费者负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it } from "vitest";

import { UiDirectoryTabs } from "@/shared/ui/navigation/directory-tabs";
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
    expect(all.className).toContain("whitespace-nowrap");
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

    const dismissButton = screen.getByRole("button", { name: "关闭当前视图" });
    expect(dismissButton.getAttribute("title")).toBe("关闭当前视图");
    await user.click(dismissButton);
    expect(dismissals).toEqual(["work"]);
    await user.keyboard("{Enter}");
    await user.keyboard(" ");
    expect(dismissals).toEqual(["work", "work", "work"]);
    expect(changes).toEqual([]);
  });

  it("renders page-level choices as a stable single-line indicator", () => {
    render(
      <UiTabs
        activeValue="global"
        ariaLabel="技能目录"
        options={[
          { label: "全局技能库", value: "global" },
          { label: "社区技能", value: "community" },
        ]}
      />,
    );

    const active = screen.getByRole("button", { name: "全局技能库" });
    const inactive = screen.getByRole("button", { name: "社区技能" });
    expect(active.className).toContain("whitespace-nowrap");
    expect(active.className).toContain("border-(--text-strong)");
    expect(active.className).not.toContain("radius-control-sm");
    expect(inactive.className).toContain("border-transparent");
  });

  it("provides a compact directory preset without knowing a business domain", async () => {
    const user = userEvent.setup();
    const changes: string[] = [];
    render(
      <UiDirectoryTabs
        activeValue="global"
        ariaLabel="资源目录"
        onChange={(value) => changes.push(value)}
        options={[
          { label: "全局资源", value: "global" },
          { label: "社区资源", value: "community" },
        ]}
      />,
    );

    const group = screen.getByRole("group", { name: "资源目录" });
    expect(group.className).toContain("w-fit");
    expect(group.className.split(/\s+/)).not.toContain("w-full");
    await user.click(screen.getByRole("button", { name: "社区资源" }));
    expect(changes).toEqual(["community"]);
  });
});
