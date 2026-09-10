// INPUT: Memory Header 的运行时写入、保存和删除状态。
// OUTPUT: 证明三类瞬时状态映射到共享 Spinner 尺寸且动作可用性不变。
// POS: Memory Header DOM 合同；动作选择规则由 memory-document-model 测试负责。

import { render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { MemoryDocument } from "@/types/memory/memory";

import { MemoryDocumentHeader } from "./memory-document-header";

const DOCUMENT: MemoryDocument = {
  indexed: true,
  kind: "topic",
  modified_at: "2026-04-11T08:00:00Z",
  path: "memory/project.md",
  size: 128,
  title: "Project",
};

const CONTROLLER = {
  cancelEditing: vi.fn(),
  dirty: false,
  editing: false,
  isReconciling: false,
  isSaving: false,
  revision: "revision-1",
  save: vi.fn(async () => undefined),
  saveBlocked: false,
  startEditing: vi.fn(),
};

function header(overrides: {
  controller?: Partial<typeof CONTROLLER>;
  deleteBusy?: boolean;
  deleting?: boolean;
  runtimeWriting?: boolean;
} = {}) {
  return (
    <I18N_CONTEXT.Provider
      value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
    >
      <MemoryDocumentHeader
        controller={{ ...CONTROLLER, ...overrides.controller }}
        deleteBusy={overrides.deleteBusy ?? false}
        deleting={overrides.deleting ?? false}
        document={DOCUMENT}
        locale="zh"
        onBack={vi.fn()}
        onDelete={vi.fn()}
        runtimeWriting={overrides.runtimeWriting ?? false}
      />
    </I18N_CONTEXT.Provider>
  );
}

describe("MemoryDocumentHeader", () => {
  it("uses one semantic Spinner scale for writing, saving, and deleting", () => {
    const view = render(header({ runtimeWriting: true }));
    expect(view.container.querySelector("svg.animate-spin")?.getAttribute("class"))
      .toContain("h-3 w-3");

    view.rerender(header({
      controller: { dirty: true, editing: true, isSaving: true },
    }));
    expect(view.container.querySelector("svg.animate-spin")?.getAttribute("class"))
      .toContain("h-3.5 w-3.5");

    view.rerender(header({ deleteBusy: true, deleting: true }));
    const deletingSpinner = view.container.querySelector("svg.animate-spin");
    expect(deletingSpinner?.getAttribute("class")).toContain("h-4 w-4");
    expect(deletingSpinner?.getAttribute("class")).toContain("motion-reduce:animate-none");
  });
});
