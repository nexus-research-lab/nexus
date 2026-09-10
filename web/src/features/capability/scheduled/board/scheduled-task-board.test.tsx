// INPUT: 定时任务快照、加载阶段与空目录动作。
// OUTPUT: 证明空态和刷新状态复用共享 Button、Typography、Spinner 与圆角 recipe。
// POS: 定时任务看板 DOM 合同；分列与状态文案归纯投影 model。

import type { ComponentProps } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { ScheduledTaskBoard } from "./scheduled-task-board";

function view(overrides: Partial<ComponentProps<typeof ScheduledTaskBoard>> = {}) {
  const props: ComponentProps<typeof ScheduledTaskBoard> = {
    failure: null,
    hasSnapshot: true,
    isLoading: false,
    isPermissionLoading: false,
    items: [],
    onConfirmDeletionStopped: vi.fn(),
    onCreate: vi.fn(),
    onCreateFromPreset: vi.fn(),
    onDelete: vi.fn(),
    onEdit: vi.fn(),
    onOpenConnector: vi.fn(),
    onOpenHistory: vi.fn(),
    onPermissionDecision: vi.fn(),
    onPermissionResume: vi.fn(),
    onRefresh: vi.fn(),
    onRunNow: vi.fn(),
    onToggleEnabled: vi.fn(),
    pending: new Map(),
    permissionFailure: null,
    unconfirmed: new Map(),
    ...overrides,
  };

  return (
    <I18N_CONTEXT.Provider
      value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
    >
      <ScheduledTaskBoard {...props} />
    </I18N_CONTEXT.Provider>
  );
}

describe("ScheduledTaskBoard", () => {
  it("renders empty suggestions through shared button and typography recipes", async () => {
    const onCreateFromPreset = vi.fn();
    const user = userEvent.setup();
    render(view({ onCreateFromPreset }));

    expect(screen.getByText("capability.scheduled_quick_start_title").className)
      .toContain("ui-type-page-title");
    expect(screen.getByText("capability.scheduled_empty_description").className)
      .toContain("ui-type-metadata");

    const suggestion = screen.getByRole("button", {
      name: /capability.scheduled_suggestion_daily_title/,
    });
    expect(suggestion.className).toContain("radius-control-sm");
    expect(suggestion.className).toContain("focus-visible:ring-2");
    await user.click(suggestion);
    expect(onCreateFromPreset).toHaveBeenCalledTimes(1);
  });

  it("uses the shared reduced-motion spinner for background refresh", () => {
    const { container } = render(view({ isLoading: true }));

    const status = screen.getByRole("status");
    expect(status.textContent).toContain("capability.scheduled_refreshing");
    expect(status.className).toContain("ui-type-caption");
    expect(status.querySelector("svg")?.className.baseVal)
      .toContain("motion-reduce:animate-none");
    expect(container.innerHTML).not.toContain("rounded-[8px]");
  });
});
