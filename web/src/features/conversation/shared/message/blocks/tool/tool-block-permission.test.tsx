// INPUT: Two views of one permission request, exact suggestion indexes and disabled explanations.
// OUTPUT: Independent native groups, controlled index changes and one named/readable scope surface.
// POS: Permission view regression; selecting a scope does not dispatch an authorization decision.

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { ToolBlockPermission } from "./tool-block-permission";
import type { ToolBlockViewModel } from "./tool-block-types";

const model: ToolBlockViewModel = {
  collapsedDetailText: null, durationText: "", expandedInputText: null, hasResult: false,
  liveStatusText: null, primaryInputDetail: { key: "command", label: "Command", value: "pwd" },
  readableSuggestions: [{ index: 3, label: "This session" }, { index: 7, label: "This workspace" }],
  status: "waiting_permission", statusText: "", statusTone: "waiting", toolTitle: "Shell",
  toolVisualKind: "terminal", waitingActionHint: "",
};

describe("ToolBlockPermission", () => {
  it("preserves the disabled explanation and full input when no reusable scopes exist", () => {
    render(<I18nProvider><ToolBlockPermission
      interactionDisabled interactionDisabledReason="Request expired"
      model={{ ...model, readableSuggestions: [] }}
      onSelectedSuggestionIndexChange={vi.fn()} selectedSuggestionIndex={-1}
    /></I18nProvider>);
    expect(screen.queryByRole("group")).toBeNull();
    expect(screen.queryByRole("radio")).toBeNull();
    expect(screen.getAllByText("Request expired")).toHaveLength(1);
    expect(screen.getByText("pwd").tagName).toBe("PRE");
  });

  it("keeps two mounted views of one request selected independently", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    const request = { request_id: "same-request", tool_input: {}, on_allow: vi.fn(), on_deny: vi.fn() };
    function Instance({ name }: { name: string }) {
      const [selected, setSelected] = useState(3);
      const props = { interactionDisabled: false, model, permissionRequest: request, selectedSuggestionIndex: selected,
        onSelectedSuggestionIndexChange: (index: number) => { setSelected(index); onSelect(name, index); } };
      return <section aria-label={name}><ToolBlockPermission {...props} /></section>;
    }
    render(<I18nProvider><Instance name="First view" /><Instance name="Second view" /></I18nProvider>);
    const first = within(screen.getByRole("region", { name: "First view" }));
    const second = within(screen.getByRole("region", { name: "Second view" }));
    const firstSession = first.getByRole("radio", { name: "This session" }) as HTMLInputElement;
    const secondSession = second.getByRole("radio", { name: "This session" }) as HTMLInputElement;
    expect(firstSession.checked).toBe(true);
    expect(secondSession.checked).toBe(true);
    await user.click(first.getByRole("radio", { name: "This workspace" }));
    expect(secondSession.checked).toBe(true);
    expect(onSelect).toHaveBeenCalledExactlyOnceWith("First view", 7);
    expect(firstSession.name).not.toBe(secondSession.name);
    expect(request.on_allow).not.toHaveBeenCalled();
    expect(request.on_deny).not.toHaveBeenCalled();
  });

  it("names the scope group, associates its disabled reason and preserves exact indexes", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    const base = { model, selectedSuggestionIndex: 3, onSelectedSuggestionIndexChange: onSelect,
      permissionRequest: { request_id: "scope", tool_input: {}, on_allow: vi.fn(), on_deny: vi.fn() } };
    const { rerender } = render(<I18nProvider><ToolBlockPermission {...base} interactionDisabled interactionDisabledReason="Already handled elsewhere" /></I18nProvider>);
    const group = screen.getByRole("group");
    expect(document.getElementById(group.getAttribute("aria-labelledby")!)?.textContent).toBeTruthy();
    expect(document.getElementById(group.getAttribute("aria-describedby")!)?.textContent).toBe("Already handled elsewhere");
    expect(screen.getAllByText("Already handled elsewhere")).toHaveLength(1);
    for (const radio of within(group).getAllByRole("radio")) {
      expect((radio as HTMLInputElement).disabled).toBe(true);
      await user.click(radio);
    }
    expect(onSelect).not.toHaveBeenCalled();
    rerender(<I18nProvider><ToolBlockPermission {...base} interactionDisabled={false} /></I18nProvider>);
    await user.click(within(group).getByRole("radio", { name: "This workspace" }));
    expect(onSelect).toHaveBeenCalledExactlyOnceWith(7);
    await user.click(within(group).getAllByRole("radio")[0]);
    expect(onSelect).toHaveBeenLastCalledWith(-1);
  });
});
