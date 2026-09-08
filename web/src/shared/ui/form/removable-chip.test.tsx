// INPUT: RemovableChip 的标签、移除命令与禁用态。
// OUTPUT: 证明可移除实体使用一个真实、具名且可禁用的按钮。
// POS: Form chip 行为测试；实体集合更新由消费者负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { UiRemovableChip } from "@/shared/ui/form/removable-chip";

describe("UiRemovableChip", () => {
  it("routes removal through one named icon button", async () => {
    const user = userEvent.setup();
    const onRemove = vi.fn();
    render(
      <UiRemovableChip onRemove={onRemove} removeLabel="移除研究" size="xs">
        研究
      </UiRemovableChip>,
    );

    const remove = screen.getByRole("button", { name: "移除研究" });
    expect(screen.getAllByRole("button")).toHaveLength(1);
    expect(document.querySelector('[role="button"]')).toBeNull();
    await user.click(remove);
    expect(onRemove).toHaveBeenCalledTimes(1);
  });

  it("keeps disabled removal in the native button contract", async () => {
    const user = userEvent.setup();
    const onRemove = vi.fn();
    render(
      <UiRemovableChip disabled onRemove={onRemove} removeLabel="移除研究">
        研究
      </UiRemovableChip>,
    );

    const remove = screen.getByRole("button", { name: "移除研究" }) as HTMLButtonElement;
    expect(remove.disabled).toBe(true);
    await user.click(remove);
    expect(onRemove).not.toHaveBeenCalled();
  });
});
