// INPUT: Tour registration changes during active navigation.
// OUTPUT: Unregistered tours close without interrupting immediate replacement or another tour.
// POS: Provider lifecycle regression; overlay rendering is isolated.
import { act, render, screen, waitFor } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { OnboardingTourProvider } from "./tour-provider";
import { useOnboardingTour } from "./use-onboarding-tour";
import type { OnboardingTourContextValue } from "./tour-contract";
const hydration = vi.hoisted(() => vi.fn(async () => ({ completedTours: {} as Record<string, boolean> })));
vi.mock("./tour-state", () => ({
  readCompletedTours: () => ({}), hydrateOnboardingStateFromDesktop: hydration,
  writeCompletedTours: vi.fn(), resetAllTourState: vi.fn(),
}));
vi.mock("./overlay/tour-overlay", () => ({ OnboardingTourOverlay: () => null }));
let api: OnboardingTourContextValue;
function Consumer() { api = useOnboardingTour(); return <output>{api.activeTourId ?? "none"}</output>; }
const tour = (id: string) => ({ id, steps: [{ id: "one", title: "Step", description: "Details" }] });

it("closes an unregistered active tour", async () => {
  render(<OnboardingTourProvider><Consumer /></OnboardingTourProvider>);
  await act(async () => { api.registerTour(tour("a")); api.startTour("a"); });
  expect(screen.getByText("a")).toBeTruthy();
  await act(async () => { api.unregisterTour("a"); });
  await waitFor(() => expect(screen.getByText("none")).toBeTruthy());
});

it("preserves same-turn registration updates and another active tour", async () => {
  render(<OnboardingTourProvider><Consumer /></OnboardingTourProvider>);
  await act(async () => { api.registerTour(tour("a")); api.startTour("a"); });
  await act(async () => { api.unregisterTour("a"); api.registerTour(tour("a")); });
  expect(screen.getByText("a")).toBeTruthy();
  await act(async () => { api.unregisterTour("a"); api.registerTour(tour("b")); api.startTour("b"); });
  expect(screen.getByText("b")).toBeTruthy();
});


it("does not restore completed tours when hydration finishes after explicit reset", async () => {
  let finish!: (value: { completedTours: Record<string, boolean> }) => void;
  hydration.mockReturnValueOnce(new Promise((resolve) => { finish = resolve; }));
  render(<OnboardingTourProvider><Consumer /></OnboardingTourProvider>);
  act(() => api.resetAllTours());
  await act(async () => finish({ completedTours: { a: true } }));
  expect(api.hasCompletedTour("a")).toBe(false);
  expect(api.isTourStateReady).toBe(true);
});
