// INPUT: Independent directory labels, values and controlled selection commands.
// OUTPUT: Category and status filters share a structure without sharing selection state.
// POS: Cross-domain filter pattern regression; overlay behavior remains owned by UiSelectMenu.

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { UiFilterSelect } from "./filter-select";

describe("UiFilterSelect", () => {
  it("keeps category and status filters structurally identical with independent selection", async () => {
    const user = userEvent.setup();
    function Filters() {
      const [category, setCategory] = useState("all");
      const [status, setStatus] = useState("all");
      return <>
        <UiFilterSelect ariaLabel="Category" onChange={setCategory}
          options={[{ label: "All categories", value: "all" }, { label: "Tools", value: "tools" }]} value={category} />
        <UiFilterSelect ariaLabel="Status" onChange={setStatus}
          options={[{ label: "All statuses", value: "all" }, { label: "Connected", value: "connected" }]} value={status} />
      </>;
    }
    render(<Filters />, { wrapper: I18nProvider });
    const category = screen.getByRole("button", { name: "Category" });
    const status = screen.getByRole("button", { name: "Status" });
    expect(category.className).toBe(status.className);
    for (const trigger of [category, status]) {
      expect(trigger.querySelectorAll("svg")).toHaveLength(1);
      expect(within(trigger).queryByText(trigger.getAttribute("aria-label")!)).toBeNull();
    }
    expect(within(category).getByText("All categories")).toBeTruthy();
    expect(within(status).getByText("All statuses")).toBeTruthy();
    await user.click(category);
    await user.click(screen.getByRole("option", { name: "Tools" }));
    expect(within(category).getByText("Tools")).toBeTruthy();
    expect(within(status).getByText("All statuses")).toBeTruthy();
    await user.click(status);
    await user.click(screen.getByRole("option", { name: "Connected" }));
    expect(within(category).getByText("Tools")).toBeTruthy();
    expect(within(status).getByText("Connected")).toBeTruthy();
  });

});
