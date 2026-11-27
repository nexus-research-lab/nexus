// INPUT: 两个入口命令（主按钮 + 次按钮）与可选的次按钮文案。
// OUTPUT: 两行的「你的 AI 队友」引导卡；聊天与联系人侧栏在只有主智能体时共用，次按钮分别是创建群聊与管理联系人。
// POS: Home 侧栏引导视图；是否展示由各侧栏面板按目录判断，本组件不读取数据。

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

/** 装饰用的三位示例队友头像，取自内置头像集，不代表真实 Agent。 */
const TEAMMATE_PREVIEW_AVATARS = ["12", "20", "33"] as const;

export function SidebarTeammateCard({
  onCreateAgent,
  onSecondary,
  secondaryLabel,
}: {
  onCreateAgent: () => void;
  onSecondary: () => void;
  secondaryLabel?: string;
}) {
  const { t } = useI18n();
  const title = t("sidebar.teammate_title");
  return (
    <section
      aria-label={title}
      className="sidebar-teammate-card surface-radius-md mt-2 flex flex-col gap-2 py-2.5 pl-2 pr-[7px]"
    >
      {/* 卡片外缘与列表行的激活底色同宽；左侧内边距与行内头像起点对齐，右侧 1px 边框加 7px 内边距让按钮右缘贴住时间右线。 */}
      {/* 两行：头像组 + 标题，下面两个小按钮固定一行平分卡片宽度；侧栏收窄时文字截断，按钮不换行也不超出卡片右边框。 */}
      <span className="flex min-w-0 items-center gap-2">
        <span aria-hidden className="flex shrink-0">
          {TEAMMATE_PREVIEW_AVATARS.map((avatar, index) => (
            <UiAgentAvatar
              avatar={avatar}
              className={index > 0 ? "sidebar-teammate-avatar -ml-2" : "sidebar-teammate-avatar"}
              key={avatar}
              name=""
              size="xs"
            />
          ))}
        </span>
        <span className={`truncate ${getUiTypographyClassName({ role: "supporting", tone: "strong", weight: "regular" })}`}>
          {title}
        </span>
      </span>
      <span className="grid grid-cols-2 gap-1.5">
        <UiButton className="min-w-0 px-1" onClick={onCreateAgent} size="2xs" tone="primary" variant="solid">
          <span className="truncate">{t("sidebar.teammate_create_agent")}</span>
        </UiButton>
        <UiButton className="min-w-0 px-1" onClick={onSecondary} size="2xs" variant="outline">
          <span className="truncate">{secondaryLabel ?? t("sidebar.teammate_create_room")}</span>
        </UiButton>
      </span>
    </section>
  );
}
