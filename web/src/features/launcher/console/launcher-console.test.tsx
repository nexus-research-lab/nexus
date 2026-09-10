// INPUT: 无Provider的查询提交与当前页面引导状态。
// OUTPUT: 配置前提交未受理，草稿不进入输入清空路径。
// POS: Launcher装配层受理合同回归，业务控制器与装饰组件隔离。
import { act, render } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import type { HeroStageProps } from "./launcher-console-types";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { LauncherConsole } from "./launcher-console";
const mocks = vi.hoisted(() => ({ updateQuery: vi.fn(), submit: vi.fn(), hero: null as HeroStageProps | null }));
vi.mock("./use-launcher-console-controller", () => ({ useLauncherConsoleController: () => ({
  state: { query: "Research draft", isQueryLoading: false },
  actions: { updateQuery: mocks.updateQuery, submitQuery: mocks.submit, enterHome: vi.fn(), openRecentEntry: vi.fn() },
}) }));
vi.mock("../hero/launcher-hero-stage", () => ({ LauncherHeroStage: (props: HeroStageProps) => { mocks.hero = props; return null; } }));
vi.mock("@/hooks/capability/use-provider-availability", () => ({ useProviderAvailability: () => ({ isReady: true, hasAvailableProvider: false }) }));
vi.mock("@/features/onboarding/provider-setup/provider-setup-dialog", () => ({ ProviderSetupDialog: () => null }));
vi.mock("@/shared/ui/onboarding/use-page-onboarding-tour", () => ({ usePageOnboardingTour: vi.fn() }));
vi.mock("@/shared/ui/onboarding/tour-state", () => ({ setTourDismissed: vi.fn() }));
vi.mock("@/shared/ui/feedback/lottie-player", () => ({ LottiePlayer: () => null }));
it("opens provider setup without accepting and clearing the draft", () => {
  render(<I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>
    <LauncherConsole initialQuery="" agents={[]} conversations={[]} rooms={[]} currentAgentId={null} onOpenMainAgentDm={vi.fn()} onOpenRoute={vi.fn()} onSelectAgent={vi.fn()} />
  </I18N_CONTEXT.Provider>);
  let accepted: boolean | undefined;
  act(() => { accepted = mocks.hero!.onSubmit("Research draft"); });
  expect(accepted).toBe(false);
  expect(mocks.updateQuery).toHaveBeenCalledExactlyOnceWith("Research draft");
  expect(mocks.submit).not.toHaveBeenCalled();
});
