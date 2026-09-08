// INPUT: Breadcrumb 的静态层级、返回命令、链接、前导身份与密度。
// OUTPUT: 证明导航语义、当前层级、共享动作和统一分隔符可被真实操作。
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
    const { container } = render(
      <UiBreadcrumb
        ariaLabel="当前位置"
        items={[
          {
            icon: <ArrowLeft />,
            id: "skills",
            label: "技能",
            onSelect: onBack,
          },
          { id: "detail", label: "研究助手" },
        ]}
      />,
    );

    expect(screen.getByRole("navigation", { name: "当前位置" })).toBeTruthy();
    expect(screen.getByText("研究助手").getAttribute("aria-current")).toBe("page");
    expect(container.querySelectorAll("[data-slot='breadcrumb-separator']")).toHaveLength(1);

    await user.click(screen.getByRole("button", { name: "技能" }));
    expect(onBack).toHaveBeenCalledOnce();
  });

  it("supports links, leading identity, compact actions, and single-line labels", () => {
    const { container } = render(
      <UiBreadcrumb
        ariaLabel="文件位置"
        density="compact"
        items={[
          { href: "/workspace", id: "workspace", label: "Nexus 工作区" },
          { id: "folder", label: "output" },
          { id: "file", label: "counter.txt" },
        ]}
        leading={<span>N</span>}
      />,
    );

    const link = screen.getByRole("link", { name: "Nexus 工作区" });
    expect(link.className).toContain("min-h-6");
    expect(link.className).toContain("whitespace-nowrap");
    expect(container.querySelectorAll("[data-slot='breadcrumb-separator']")).toHaveLength(3);
    expect(container.querySelector("[data-slot='breadcrumb-leading']")?.textContent).toBe("N");
    expect(container.querySelector("[data-current='true']")?.className).toContain("basis-2/5");
    expect(container.querySelector("[data-current='true']")?.className).not.toContain("flex-1");
    expect(screen.getByText("counter.txt").getAttribute("aria-current")).toBe("page");
  });
});
