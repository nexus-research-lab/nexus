// INPUT: 部分部署失败、空明细及中英文反馈。
// OUTPUT: 证明反馈保留准确数量、有限真实姓名/通称，不泄露内部身份或原始错误。
// POS: 技能操作控制器的部分成功反馈回归。

import { describe, expect, it } from "vitest";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { formatDeployFailureMessage } from "./skill-deploy-failures";

describe("skill deployment names", () => {
  it.each(["zh", "en"] as const)("retains partial success and the exact failure count in %s", (locale) => {
    const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {}).reduce((value, [name, replacement]) => value.replaceAll(`{${name}}`, String(replacement)), MESSAGES[locale][key]);
    const failures = [" Nova ", " ", "", "Pixel"].map((agent_name, index) => ({ agent_id: `private-agent-${index}`, agent_name, error: "private runtime error" }));
    expect(formatDeployFailureMessage("Analysis", failures, { locale, t })).toBe(t("capability.skills_deploy_failed", {
      name: "Analysis", count: 4,
      agents: t("capability.skills_agent_list_more", { agents: ["Nova", t("agent.name_fallback"), t("agent.name_fallback")].join(locale === "zh" ? "、" : ", ") }),
    }));
    expect(failures[0].agent_name).toBe(" Nova ");
    expect(formatDeployFailureMessage("Analysis", undefined, { locale, t })).toBeNull();
    expect(formatDeployFailureMessage("Analysis", [{ agent_id: "", agent_name: "", error: "" }], { locale, t })).toBeNull();
  });
});
