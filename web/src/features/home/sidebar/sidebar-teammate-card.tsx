// INPUT: 新建 Agent 与新建群聊两个入口命令。
// OUTPUT: 还没有自建 Agent 时，聊天列表下方「你的 AI 队友」引导卡。
// POS: Home 侧栏引导视图；是否展示由侧栏面板按目录判断，本组件不读取数据。

import { Plus } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

/** 装饰用的三位示例队友头像，取自内置头像集，不代表真实 Agent。 */
const TEAMMATE_PREVIEW_AVATARS = ["12", "20", "33"] as const;

export function SidebarTeammateCard({
  onCreateAgent,
  onCreateRoom,
}: {
  onCreateAgent: () => void;
  onCreateRoom: () => void;
}) {
  const { t } = useI18n();
  return (
    <section
      aria-label={t("sidebar.teammate_title")}
      className="sidebar-teammate-card surface-radius-md mx-0.5 mt-2 flex flex-col gap-2.5 px-3 pb-3 pt-3"
    >
      {/* 侧栏较窄：头像、标题、说明纵向排列，动作按钮占满整行。 */}
      <span aria-hidden className="flex">
        {TEAMMATE_PREVIEW_AVATARS.map((avatar, index) => (
          <UiAgentAvatar
            avatar={avatar}
            className={index > 0 ? "sidebar-teammate-avatar -ml-2" : "sidebar-teammate-avatar"}
            key={avatar}
            name=""
            size="sm"
          />
        ))}
      </span>
      <span className="flex min-w-0 flex-col gap-0.5">
        <span className={getUiTypographyClassName({ role: "control", tone: "strong", weight: "semibold" })}>
          {t("sidebar.teammate_title")}
        </span>
        <span className={getUiTypographyClassName({ role: "caption", tone: "soft" })}>
          {t("sidebar.teammate_description")}
        </span>
      </span>
      <span className="flex flex-col gap-1.5">
        <UiButton className="w-full" onClick={onCreateAgent} size="sm" tone="primary" variant="solid">
          <Plus aria-hidden className="h-3.5 w-3.5" />
          {t("sidebar.teammate_create_agent")}
        </UiButton>
        <UiButton className="w-full" onClick={onCreateRoom} size="sm" variant="outline">
          {t("home.rooms_empty_action")}
        </UiButton>
      </span>
    </section>
  );
}
