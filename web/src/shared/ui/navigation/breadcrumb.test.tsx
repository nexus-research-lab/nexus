// INPUT: Breadcrumb 的路径层级、返回命令与导航链接。
// OUTPUT: 证明当前页标记、返回动作和链接目标。
// POS: UiBreadcrumb DOM 行为测试；路由更新与业务路径生成由消费者负责。

import { ArrowLeft } from "lucide-react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { UiBreadcrumb } from "@/shared/ui/navigation/breadcrumb";

describe("UiBreadcrumb", () => {
  it("renders one named navigation trail with one current page", async () => {
    const user = userEvent.setup();
    const onBack = vi.fn();
    render(
      <UiBreadcrumb
        ariaLabel="当前位置"
        items={[
          {
            icon: <ArrowLeft />,
            id: "skills",
            label: "技能",
            onSelect: onBack,
          },
          { href: "/workspace", id: "workspace", label: "工作区" },
          { id: "detail", label: "研究助手" },
        ]}
      />,
    );

    expect(screen.getByRole("navigation", { name: "当前位置" })).toBeTruthy();
    expect(screen.getByText("研究助手").getAttribute("aria-current")).toBe("page");
    expect(screen.getByRole("link", { name: "工作区" }).getAttribute("href")).toBe("/workspace");

    await user.click(screen.getByRole("button", { name: "技能" }));
    expect(onBack).toHaveBeenCalledOnce();
  });

});
