// INPUT: Bundled and user-provided Skill entries plus both language catalogs.
// OUTPUT: Bundled Skills read in the active language; user Skills keep their own metadata.
// POS: Skill display projection regression.
import { describe, expect, it } from "vitest";

import { MESSAGES, type Locale, type TranslationKey } from "@/shared/i18n/messages";
import type { SkillInfo } from "@/types/capability/skill";

import {
  getSkillDisplayDescription,
  getSkillDisplayTitle,
} from "./skill-description";

const translate = (locale: Locale) => (key: TranslationKey) => MESSAGES[locale][key];

function skill(name: string, overrides: Partial<SkillInfo> = {}): SkillInfo {
  return {
    name,
    title: "原始标题",
    description: "原始说明",
    source_type: "builtin",
    source_kind: "nexus_platform",
    ...overrides,
  } as SkillInfo;
}

describe("skill display projection", () => {
  it.each([
    ["ima-skill", "IMA notes and knowledge base", "IMA 笔记与知识库"],
    ["wechat-article-search", "WeChat article search", "微信公众号文章搜索"],
    ["caixun-work-hours", "Caixun timesheets", "彩讯工时填报"],
  ])("localizes the bundled %s title and description", (name, english, chinese) => {
    const item = skill(name);
    expect(getSkillDisplayTitle(item, translate("en"))).toBe(english);
    expect(getSkillDisplayTitle(item, translate("zh"))).toBe(chinese);
    expect(getSkillDisplayDescription(item, translate("en"))).not.toMatch(/\p{Script=Han}/u);
  });

  it.each(["visualize", "computer-use"])(
    "localizes the bundled %s description and keeps its Chinese copy",
    (name) => {
      const item = skill(name);
      expect(getSkillDisplayDescription(item, translate("en"))).not.toMatch(/\p{Script=Han}/u);
      expect(getSkillDisplayDescription(item, translate("zh"))).toMatch(/\p{Script=Han}/u);
      expect(getSkillDisplayDescription(item, translate("zh"))).not.toBe("原始说明");
    },
  );

  it("keeps a user-provided Skill with a bundled name unchanged", () => {
    const item = skill("ima-skill", { source_type: "external", source_kind: "user" });
    expect(getSkillDisplayTitle(item, translate("en"))).toBe("原始标题");
    expect(getSkillDisplayDescription(item, translate("en"))).toBe("原始说明");
  });
});
