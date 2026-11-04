// INPUT: Action portal targets, detail navigation and brand names.
// OUTPUT: Target migration and cleanup, navigation and accessible names.
// POS: Shared capability layout behavior.

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { CapabilityBrandIcon } from "./capability-brand-icon";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { PageHeaderActionsContext } from "@/shared/lib/react/page-header-actions-context";

import {
  CapabilityDetailPage,
  CapabilityPageLayout,
} from "./capability-page-layout";

describe("CapabilityPageLayout", () => {

  it("moves the action with the host target and restores its inline owner when the target disappears", () => {
    const firstTarget = document.createElement("div");
    const nextTarget = document.createElement("div");
    let actions = 0;
    const view = (target: HTMLElement | null) => (
      <PageHeaderActionsContext.Provider value={target}>
        <CapabilityPageLayout
          actions={<button onClick={() => { actions += 1; }} type="button">Add connector</button>}
          title="Connectors"
        >
          <div>Catalog</div>
        </CapabilityPageLayout>
      </PageHeaderActionsContext.Provider>
    );
    const { container, rerender, unmount } = render(view(firstTarget));
    expect(firstTarget.querySelector("button")?.textContent).toBe("Add connector");
    expect(container.querySelector("button")).toBeNull();
    fireEvent.click(firstTarget.querySelector("button")!);
    expect(actions).toBe(1);

    rerender(view(nextTarget));
    expect(firstTarget.childElementCount).toBe(0);
    expect(nextTarget.querySelector("button")?.textContent).toBe("Add connector");
    fireEvent.click(nextTarget.querySelector("button")!);
    expect(actions).toBe(2);

    rerender(view(null));
    expect(nextTarget.childElementCount).toBe(0);
    fireEvent.click(screen.getByRole("button", { name: "Add connector" }));
    expect(actions).toBe(3);
    unmount();
    expect(firstTarget.childElementCount).toBe(0);
    expect(nextTarget.childElementCount).toBe(0);
  });

  it("owns one detail page axis and current-object navigation contract", () => {
    let backCount = 0;
    render(
<I18N_CONTEXT.Provider
value={{ locale: "en",setLocale: () => undefined,t: (key) => key }}
>
<CapabilityDetailPage
backLabel="Skills"
currentTitle="Research Skill"
onBack={() => { backCount+=1; } }
>
<div>Detail content</div>
</CapabilityDetailPage>
</I18N_CONTEXT.Provider>
);

    fireEvent.click(screen.getByRole("button", { name: "Skills" }));

    expect(backCount).toBe(1);

    expect(screen.getByRole("navigation", { name: "common.location_aria" })).toBeTruthy();

    expect(screen.getByText("Research Skill").getAttribute("aria-current")).toBe("page");
    expect(screen.getByText("Detail content")).toBeTruthy();
  });

});

it("names capability artwork with image semantics without exposing decorative children", () => {
  const { rerender } = render(<CapabilityBrandIcon title="Mail" src="/mail.svg" />);
  expect(screen.getByRole("img", { name: "Mail" })).toBeTruthy();
  rerender(<CapabilityBrandIcon title="Mail" fallback="M" />);
  expect(screen.getByRole("img", { name: "Mail" })).toBeTruthy();
  expect(screen.getByText("M").getAttribute("aria-hidden")).toBe("true");
});
