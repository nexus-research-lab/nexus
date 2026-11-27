// INPUT: Real file-artifact DOM, explicit/missing scope and independent open/download capabilities.
// OUTPUT: Exact-scope commands, no ambient fallback, localized missing identity and label suppression.
// POS: File Artifact integration tests; only the API transport is replaced with a local spy.

import { act, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { useAgentStore } from "@/store/agent";
import { FileArtifactBlock } from "./file-artifact-block";

vi.mock("@/lib/api/agent/agent-api", () => ({ downloadWorkspaceFileApi: vi.fn().mockResolvedValue(undefined) }));

afterEach(() => {
  act(() => useAgentStore.setState({ current_agent_id: null }));
  vi.clearAllMocks();
});

describe("File Artifact", () => {

  it("updates current-language missing-path explanations and localized default names", () => {
    const view = (locale: I18nContextValue["locale"]) => (
      <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => MESSAGES[locale][key] }}>
        <FileArtifactBlock label="" path="  " workspaceAgentId="author" onOpenWorkspaceFile={vi.fn()} />
      </I18N_CONTEXT.Provider>
    );
    const { rerender } = render(view("zh"));
    expect(screen.getByRole("button", { name: /^文件/ }).hasAttribute("disabled")).toBe(true);
    expect(screen.getByText("缺少文件路径，暂时无法打开此文件。")).toBeTruthy();
    rerender(view("en"));
    expect(screen.getByRole("button", { name: /^file/ }).hasAttribute("disabled")).toBe(true);
    expect(screen.getByText("The file path is unavailable, so this file cannot be opened.")).toBeTruthy();
    expect(screen.getAllByRole("button")).toHaveLength(1);
  });
});
