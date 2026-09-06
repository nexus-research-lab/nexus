// INPUT: Independent directory labels, values and controlled selection commands.
// OUTPUT: Category and status filters share a structure without sharing selection state.
// POS: Cross-domain filter pattern regression; overlay behavior remains owned by UiSelectMenu.

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it } from "vitest";
import { UiFilterSelect } from "./filter-select";

describe("UiFilterSelect", () => {
  it("keeps category and status filters structurally identical with independent selection", async () => {
    const user = userEvent.setup();
    function Filters() {
      const [category, setCategory] = useState("all");
      const [status, setStatus] = useState("all");
      return <>
        <UiFilterSelect ariaLabel="Category" label="Category" onChange={setCategory}
          options={[{ label: "All", value: "all" }, { label: "Tools", value: "tools" }]} value={category} />
        <UiFilterSelect ariaLabel="Status" label="Status" onChange={setStatus}
          options={[{ label: "All", value: "all" }, { label: "Connected", value: "connected" }]} value={status} />
      </>;
    }
    render(<Filters />);
    const category = screen.getByRole("button", { name: "Category" });
    const status = screen.getByRole("button", { name: "Status" });
    expect(category.className).toBe(status.className);
    for (const trigger of [category, status]) {
      expect(trigger.querySelectorAll("svg")).toHaveLength(1);
      expect(within(trigger).getByText("All")).toBeTruthy();
    }
    await user.click(category);
    await user.click(screen.getByRole("option", { name: "Tools" }));
    expect(within(category).getByText("Tools")).toBeTruthy();
    expect(within(status).getByText("All")).toBeTruthy();
    await user.click(status);
    await user.click(screen.getByRole("option", { name: "Connected" }));
    expect(within(category).getByText("Tools")).toBeTruthy();
    expect(within(status).getByText("Connected")).toBeTruthy();
  });

});
