// INPUT: 新建 Agent 与新建群聊两个入口命令。
// OUTPUT: 还没有自建 Agent 时，聊天列表下方两行的「你的 AI 队友」引导卡。
// POS: Home 侧栏引导视图；是否展示由侧栏面板按目录判断，本组件不读取数据。

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
      className="sidebar-teammate-card surface-radius-md mt-2 flex flex-col gap-2 px-2.5 py-2.5"
    >
      {/* 两行：头像组 + 标题，下面两个动作均分一行；侧栏约 176px，按钮文案保持短词不换行。 */}
      <span className="flex min-w-0 items-center gap-2">
        <span aria-hidden className="flex shrink-0">
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
        <span className={`truncate ${getUiTypographyClassName({ role: "control", tone: "strong", weight: "semibold" })}`}>
          {t("sidebar.teammate_title")}
        </span>
      </span>
      <span className="flex gap-1.5">
        <UiButton className="flex-auto whitespace-nowrap px-1.5" onClick={onCreateAgent} size="xs" tone="primary" variant="solid">
          {t("sidebar.teammate_create_agent")}
        </UiButton>
        <UiButton className="flex-auto whitespace-nowrap px-1.5" onClick={onCreateRoom} size="xs" variant="outline">
          {t("sidebar.teammate_create_room")}
        </UiButton>
      </span>
    </section>
  );
}
