// INPUT: Initial catalog failure, a successful snapshot and a failed read-only refresh.
// OUTPUT: Never presents failure as empty; retained cards stay visible while refreshing and after failure.
// POS: Catalog controller/view recovery integration.
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { useSkillCatalog } from "../controller/use-skill-catalog";
import { SkillsCatalogGrid } from "./skills-catalog-grid";

const fixtures = vi.hoisted(() => ({ load: vi.fn(), onError: vi.fn(), t: (key: string) => key }));
vi.mock("@/lib/api/capability/skill-api", () => ({ getAvailableSkillsApi: fixtures.load }));
vi.mock("@/shared/i18n/i18n-context", () => ({ useI18n: () => ({ t: fixtures.t, locale: "en" }) }));
function Harness() {
  const catalog = useSkillCatalog({ active: true, onError: fixtures.onError });
  return <>
    <button onClick={() => void catalog.refresh()}>Read catalog</button>
    <SkillsCatalogGrid groupedSkills={catalog.groupedSkills} loading={catalog.loading} loadFailed={catalog.loadFailed}
      onReload={() => void catalog.refresh()} busySkillNames={new Set()} onDeleteSkill={vi.fn()} onOpenSkill={vi.fn()} />
  </>;
}
it("distinguishes failed reads and keeps the last cards available during recovery", async () => {
  fixtures.load.mockRejectedValueOnce(new Error("offline")).mockResolvedValueOnce([
    { name: "project-skill", title: "Project Skill", category_key: "custom", category_name: "Custom", description: "Project instructions", source_type: "external", deletable: false },
  ]);
  render(<Harness />);
  await screen.findByText("capability.skills_catalog_load_failed_title");
  expect(screen.queryByText("capability.skills_empty_title")).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "common.refresh" }));
  await screen.findByRole("button", { name: "Project Skill" });
  let rejectRead!: (error: Error) => void;
  fixtures.load.mockImplementationOnce(() => new Promise((_resolve, reject) => { rejectRead = reject; }));
  fireEvent.click(screen.getByRole("button", { name: "Read catalog" }));
  expect(screen.getByRole("button", { name: "Project Skill" })).toBeTruthy();
  rejectRead(new Error("offline again"));
  await waitFor(() => expect(screen.getByText("capability.skills_catalog_load_failed_title")).toBeTruthy());
  expect(screen.getByRole("button", { name: "Project Skill" })).toBeTruthy();
  expect(fixtures.load).toHaveBeenCalledTimes(3);
});
