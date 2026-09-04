// INPUT: shared/ui 公开 primitive/pattern 与当前 theme、locale、viewport 状态。
// OUTPUT: 可交互、可固定 URL、可在真实浏览器截图的组件状态陈列面。
// POS: 开发期视觉验收工具；只消费公共 UI，不定义或导出产品组件。

import {
  Check,
  ChevronDown,
  CircleAlert,
  Copy,
  Ellipsis,
  Layers3,
  LoaderCircle,
  MousePointer2,
  Palette,
  Plus,
  RefreshCw,
  Search,
  Sparkles,
  Trash2,
} from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { Locale } from "@/shared/i18n/messages";
import { useTheme, type VisualTheme } from "@/shared/theme/theme-context";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { UiSplitButton } from "@/shared/ui/button/split-button";
import { cn } from "@/shared/ui/class-name";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
  type UiDialogSize,
} from "@/shared/ui/dialog/dialog";
import type { UiDialogViewport } from "@/shared/ui/dialog/dialog-layout";
import { PromptDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { UiAgentAvatar, UiRoomAvatar } from "@/shared/ui/display/avatar";
import { UiBadge, UiCounterBadge } from "@/shared/ui/display/badge";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { UiCheckbox } from "@/shared/ui/form/checkbox";
import { UiChoiceButton } from "@/shared/ui/form/choice";
import {
  UiField,
  UiInput,
  UiNativeSelect,
  UiSearchInput,
  UiTextarea,
} from "@/shared/ui/form/form-control";
import { UiSegmentedControl } from "@/shared/ui/form/segmented-control";
import { UiListActionButton } from "@/shared/ui/list/list-action";
import { UiListRow } from "@/shared/ui/list/list-row";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { UiTabs } from "@/shared/ui/navigation/tabs";
import { UiTooltip } from "@/shared/ui/overlay/tooltip";
import { UiPanel } from "@/shared/ui/panel";
import {
  getUiTypographyClassName,
  type UiTypographyRole,
  type UiTypographyTone,
} from "@/shared/ui/typography/typography-styles";

import {
  ContentGallery,
  CoverageInventory,
  FoundationCompleteness,
  InteractionGallery,
  WorkspaceGallery,
} from "./ui-gallery-additional-sections";
import { galleryText } from "./ui-gallery-copy";

const THEME_OPTIONS: Array<{ label: string; value: VisualTheme }> = [
  { label: "Light", value: "light" },
  { label: "Dark", value: "dark" },
  { label: "Rain", value: "rain" },
];

const LOCALE_OPTIONS: Array<{ label: string; value: Locale }> = [
  { label: "中文", value: "zh" },
  { label: "English", value: "en" },
];

const ROOM_MEMBERS = [
  { id: "lin", name: "Lin" },
  { id: "maya", name: "Maya" },
  { id: "noah", name: "Noah" },
];

type GalleryTab = "foundation" | "content" | "interaction" | "workspace" | "coverage";

const GALLERY_TABS: readonly GalleryTab[] = [
  "foundation",
  "content",
  "interaction",
  "workspace",
  "coverage",
];

function getInitialGalleryTab(): GalleryTab {
  const section = new URLSearchParams(window.location.search).get("section");
  return GALLERY_TABS.includes(section as GalleryTab)
    ? section as GalleryTab
    : "foundation";
}

export function UiContractGallery() {
  const { locale, setLocale } = useI18n();
  const { theme, setTheme } = useTheme();
  const [activeTab, setActiveTab] = useState<GalleryTab>(getInitialGalleryTab);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [dialogViewport, setDialogViewport] = useState<UiDialogViewport>("adaptiveMax");
  const [promptOpen, setPromptOpen] = useState(false);
  const [searchValue, setSearchValue] = useState("shared/ui");
  const [selectedChoice, setSelectedChoice] = useState("balanced");
  const [selectedModel, setSelectedModel] = useState("fast");
  const [selectedRole, setSelectedRole] = useState("member");
  const [viewport, setViewport] = useState(() => window.innerWidth);
  const visualTheme: VisualTheme = theme === "sunny" ? "light" : theme;

  useEffect(() => {
    const onResize = () => setViewport(window.innerWidth);
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);

  useEffect(() => {
    const query = new URLSearchParams(window.location.search);
    query.set("theme", visualTheme);
    query.set("locale", locale);
    query.set("section", activeTab);
    window.history.replaceState(null, "", `${window.location.pathname}?${query}`);
    document.documentElement.lang = locale === "zh" ? "zh-CN" : "en";
  }, [activeTab, locale, visualTheme]);

  return (
    <main
      className="relative h-dvh overflow-y-auto bg-(--background) text-(--foreground)"
      data-gallery-locale={locale}
      data-gallery-theme={visualTheme}
      data-gallery-viewport={viewport}
    >
      <div className="relative mx-auto w-full max-w-[1480px] px-4 py-5 sm:px-6 sm:py-7 lg:px-10 lg:py-10">
        <header className="mb-6 grid gap-5 border-b border-(--divider-subtle-color) pb-6 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-end">
          <div className="min-w-0">
            <div className="mb-3 flex flex-wrap items-center gap-2">
              <UiBadge tone="primary">INTERNAL · UI CONTRACT</UiBadge>
              <UiBadge tone="running">{viewport}px viewport</UiBadge>
              <UiBadge tone="success" showDot>shared/ui live</UiBadge>
            </div>
            <h1 className={cn(
              "max-w-4xl",
              getUiTypographyClassName({ role: "display", tone: "strong" }),
            )}>
              Nexus UI Contract Gallery
            </h1>
            <p className={cn(
              "mt-2 max-w-3xl",
              getUiTypographyClassName({ role: "body", tone: "muted" }),
            )}>
              {galleryText(locale, "这里展示真实公共组件，不使用页面级替身。修改 token、recipe 或 primitive 后，先在三主题和窄窗口下检查这里，再进入业务页面。", "This gallery renders the real shared components without page-level substitutes. After changing a token, recipe, or primitive, verify all three themes and the narrow viewport here before checking feature pages.")}
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-2 lg:justify-end">
            <UiSegmentedControl
              icon={Palette}
              onChange={setTheme}
              options={THEME_OPTIONS}
              title={galleryText(locale, "视觉主题", "Visual theme")}
              value={visualTheme}
            />
            <UiSegmentedControl
              onChange={setLocale}
              options={LOCALE_OPTIONS}
              title={galleryText(locale, "界面语言", "Interface locale")}
              value={locale}
            />
          </div>
        </header>

        <div className="mb-6 flex min-w-0 items-center justify-between gap-4">
          <UiTabs
            activeValue={activeTab}
            ariaLabel={galleryText(locale, "组件预览栏目", "Gallery sections")}
            onChange={setActiveTab}
            options={[
              { label: galleryText(locale, "基础", "Foundation"), value: "foundation" },
              { label: galleryText(locale, "内容与反馈", "Content & feedback"), value: "content" },
              { label: galleryText(locale, "交互与浮层", "Interaction & overlays"), value: "interaction" },
              { label: "Workspace", value: "workspace" },
              { label: galleryText(locale, "覆盖清单", "Coverage index"), value: "coverage" },
            ]}
            variant="line"
          />
          <span className={cn(
            "hidden md:inline",
            getUiTypographyClassName({ role: "caption", tone: "soft" }),
          )}>
            {galleryText(locale, "Tab 聚焦 · Hover 检查 · Escape 关闭浮层", "Tab focus · Hover states · Escape closes overlays")}
          </span>
        </div>

        {activeTab === "foundation" ? (
        <>
        <div className="grid items-start gap-5 xl:grid-cols-2" data-gallery-panel="foundation">
          <GallerySection
            className="xl:col-span-2"
            description={galleryText(locale, "字号、行高、默认字重和字距由语义 role 一次定义；页面只选择内容层级与文字 tone。", "Semantic roles define size, line height, weight, and tracking once; pages only select hierarchy and tone.")}
            eyebrow="01 · TYPE"
            title={galleryText(locale, "字体层级", "Typography hierarchy")}
          >
            <div className="grid gap-x-8 gap-y-4 md:grid-cols-2 xl:grid-cols-3" data-gallery-typography>
              <TypographySample label="Display · 36/40" textRole="display">{galleryText(locale, "协作有序发生", "Collaboration, clearly orchestrated")}</TypographySample>
              <TypographySample label="Feature title · 24/32" textRole="featureTitle">{galleryText(locale, "前端设计系统", "Frontend design system")}</TypographySample>
              <TypographySample label="Object title · 20/28" textRole="objectTitle">{galleryText(locale, "Nexus 工作区", "Nexus workspace")}</TypographySample>
              <TypographySample label="Page title · 16/20" textRole="pageTitle">{galleryText(locale, "界面与交互", "Interface and interaction")}</TypographySample>
              <TypographySample label="Section title · 14/20" textRole="sectionTitle">{galleryText(locale, "基础组件", "Foundation components")}</TypographySample>
              <TypographySample label="Body · 14/24" textRole="body">{galleryText(locale, "普通正文用于稳定阅读和必要说明。", "Body copy supports comfortable reading and essential guidance.")}</TypographySample>
              <TypographySample label="Supporting · 13/20" textRole="supporting" tone="muted">{galleryText(locale, "辅助信息不与主要内容争夺注意力。", "Supporting information stays quieter than primary content.")}</TypographySample>
              <TypographySample label="Metadata · 12/18" textRole="metadata" tone="muted">{galleryText(locale, "更新于 2 分钟前 · 3 个成员", "Updated 2 minutes ago · 3 members")}</TypographySample>
              <TypographySample label="Caption · 11/16" textRole="caption" tone="soft">{galleryText(locale, "只用于计数和次级元数据", "Reserved for counts and secondary metadata")}</TypographySample>
              <TypographySample label="Overline · 10/16" textRole="overline" tone="brand">Foundation</TypographySample>
              <TypographySample label="Code · 13/20" textRole="code" tone="default">workspace/ui-contract</TypographySample>
            </div>
          </GallerySection>

          <GallerySection
            description={galleryText(locale, "主动作不靠阴影表达；tone 只表达语义，variant 只表达表面层级。", "Primary actions do not rely on shadows; tone expresses meaning while variant expresses surface hierarchy.")}
            eyebrow="02 · ACTIONS"
            title={galleryText(locale, "按钮与动作", "Buttons & actions")}
          >
            <GalleryRow label="Primary">
              <UiButton tone="primary" variant="solid"><Plus className="h-4 w-4" />{galleryText(locale, "新建会话", "New conversation")}</UiButton>
              <UiButton tone="primary" variant="surface">{galleryText(locale, "应用修改", "Apply changes")}</UiButton>
              <UiButton tone="primary" variant="ghost">{galleryText(locale, "查看详情", "View details")}</UiButton>
              <UiButton tone="primary" variant="text">{galleryText(locale, "了解规范", "Read guidelines")}</UiButton>
            </GalleryRow>
            <GalleryRow label="Neutral / danger">
              <UiButton variant="surface">{galleryText(locale, "取消", "Cancel")}</UiButton>
              <UiButton variant="ghost"><Copy className="h-4 w-4" />{galleryText(locale, "复制", "Copy")}</UiButton>
              <UiButton tone="danger" variant="surface"><Trash2 className="h-4 w-4" />{galleryText(locale, "删除", "Delete")}</UiButton>
              <UiButton disabled>{galleryText(locale, "不可用", "Unavailable")}</UiButton>
            </GalleryRow>
            <GalleryRow label="Busy / icon">
              <UiButton aria-busy disabled tone="primary" variant="solid">
                <LoaderCircle className="h-4 w-4 animate-spin motion-reduce:animate-none" />{galleryText(locale, "保存中", "Saving")}
              </UiButton>
              <UiIconButton aria-label={galleryText(locale, "刷新组件快照", "Refresh component snapshot")} tooltipShortcut="R" variant="surface"><RefreshCw className="h-4 w-4" /></UiIconButton>
              <UiIconButton aria-label={galleryText(locale, "更多操作", "More actions")} tooltip={galleryText(locale, "更多操作", "More actions")} variant="ghost"><Ellipsis className="h-4 w-4" /></UiIconButton>
              <UiCounterBadge count={108} />
            </GalleryRow>
            <GalleryRow label="Split action">
              <UiSplitButton
                ariaLabel={galleryText(locale, "允许操作", "Allow action")}
                className="w-28"
                mainAction={{ children: galleryText(locale, "允许本次", "Allow once") }}
                menuAction={{
                  "aria-expanded": false,
                  "aria-haspopup": "menu",
                  "aria-label": galleryText(locale, "选择允许范围", "Choose allow scope"),
                  children: <ChevronDown aria-hidden="true" className="h-4 w-4" />,
                }}
              />
            </GalleryRow>
          </GallerySection>

          <GallerySection
            description={galleryText(locale, "输入壳、校验、布尔选择和互斥选择分别由稳定 primitive 持有。", "Stable primitives separately own input shells, validation, boolean choices, and mutually exclusive selection.")}
            eyebrow="03 · INPUT"
            title={galleryText(locale, "表单与选择", "Forms & selection")}
          >
            <div className="grid gap-4 sm:grid-cols-2">
              <UiField description={galleryText(locale, "辅助说明只出现一次。", "Supporting guidance appears only once.")} htmlFor="gallery-name" label={galleryText(locale, "名称", "Name")}>
                <UiInput key={locale} defaultValue={galleryText(locale, "前端基础治理", "Frontend foundation")} id="gallery-name" variant="surface" />
              </UiField>
              <UiField error={galleryText(locale, "名称不能与现有配置重复。", "The name must not duplicate an existing configuration.")} htmlFor="gallery-error" label={galleryText(locale, "错误状态", "Error state")}>
                <UiInput key={locale} aria-invalid defaultValue={galleryText(locale, "重复名称", "Duplicate name")} id="gallery-error" variant="surface" />
              </UiField>
            </div>
            <UiField label={galleryText(locale, "搜索", "Search")}>
              <UiSearchInput onChange={setSearchValue} value={searchValue} />
            </UiField>
            <div className="grid gap-4 sm:grid-cols-3">
              <UiField label={galleryText(locale, "模型", "Model")}>
                <UiSelectMenu
                  ariaLabel={galleryText(locale, "选择模型", "Choose model")}
                  onChange={setSelectedModel}
                  options={[
                    { label: galleryText(locale, "默认对话模型", "Default conversation model"), value: "default" },
                    { label: galleryText(locale, "快速响应模型", "Fast response model"), value: "fast", badge: galleryText(locale, "推荐", "Recommended") },
                    { label: galleryText(locale, "暂不可用的模型", "Temporarily unavailable model"), value: "offline", disabled: true },
                  ]}
                  value={selectedModel}
                />
              </UiField>
              <UiField label={galleryText(locale, "备注", "Notes")}>
                <UiTextarea key={locale} defaultValue={galleryText(locale, "长内容需要在 320px 下保持可读，不横向溢出。", "Long content must remain readable at 320px without horizontal overflow.")} variant="surface" />
              </UiField>
              <UiField htmlFor="gallery-native-role" label={galleryText(locale, "原生角色", "Native role")}>
                <UiNativeSelect
                  id="gallery-native-role"
                  onChange={(event) => setSelectedRole(event.target.value)}
                  value={selectedRole}
                  variant="surface"
                >
                  <option value="member">{galleryText(locale, "成员", "Member")}</option>
                  <option value="admin">{galleryText(locale, "管理员", "Administrator")}</option>
                </UiNativeSelect>
              </UiField>
            </div>
            <GalleryRow label="Selection">
              {["fast", "balanced", "precise"].map((value) => (
                <UiChoiceButton
                  active={selectedChoice === value}
                  key={value}
                  onClick={() => setSelectedChoice(value)}
                  shape="pill"
                  choiceSize="sm"
                >
                  {value}
                </UiChoiceButton>
              ))}
              <label className="inline-flex min-h-8 items-center gap-2 text-sm text-(--text-default)" htmlFor="gallery-remember">
                <UiCheckbox defaultChecked id="gallery-remember" />{galleryText(locale, "记住选择", "Remember selection")}
              </label>
            </GalleryRow>
          </GallerySection>

          <GallerySection
            description={galleryText(locale, "身份、状态与选中态使用统一标记；列表动作不改变整行语义。", "Identity, status, and selection use shared markers; row actions do not change the row's meaning.")}
            eyebrow="04 · DISPLAY"
            title={galleryText(locale, "身份与导航", "Identity & navigation")}
          >
            <GalleryRow label="Badges">
              <UiBadge>Default</UiBadge>
              <UiBadge tone="primary">Primary</UiBadge>
              <UiBadge tone="running" showDot>Running</UiBadge>
              <UiBadge tone="success">Success</UiBadge>
              <UiBadge tone="warning">Warning</UiBadge>
              <UiBadge tone="danger">Danger</UiBadge>
            </GalleryRow>
            <GalleryRow label="Avatars">
              <UiAgentAvatar name="Nexus" size="sm" />
              <UiAgentAvatar isWorking name="Maya Chen" />
              <UiRoomAvatar members={ROOM_MEMBERS} roomId="ui-contract" title="UI contract review" />
            </GalleryRow>
            <div className="space-y-1">
              <UiListRow
                active
                actions={<UiListActionButton aria-label={galleryText(locale, "条目更多操作", "More row actions")} visibility="visible"><Ellipsis className="h-4 w-4" /></UiListActionButton>}
                description={galleryText(locale, "当前选中态不使用浮起阴影", "The active state does not use an elevated shadow")}
                leading={<UiAgentAvatar name="Lin" size="sm" />}
                meta={<UiBadge size="xs" tone="active">Active</UiBadge>}
                onClick={() => undefined}
                title={galleryText(locale, "组件所有权检查", "Component ownership audit")}
              />
              <UiListRow
                description={galleryText(locale, "长中文说明保持可读，不挤压动作列", "Long English metadata stays readable without breaking the action column")}
                leading={<UiAgentAvatar name="Noah" size="sm" />}
                onClick={() => undefined}
                title={galleryText(locale, "响应式文案检查", "Responsive content audit")}
              />
            </div>
            <div className="grid gap-3 sm:grid-cols-3">
              <UiPanel padding="sm" variant="card"><SurfaceSample label="card" /></UiPanel>
              <UiPanel padding="sm" variant="dashed"><SurfaceSample label="dashed" /></UiPanel>
              <UiPanel padding="sm" variant="plain"><SurfaceSample label="plain" /></UiPanel>
            </div>
          </GallerySection>

          <GallerySection
            description={galleryText(locale, "加载、空、失败和完成是资源状态，不由页面临时拼接颜色与动作。", "Loading, empty, failure, and completion are resource states rather than page-level color and action recipes.")}
            eyebrow="05 · FEEDBACK"
            title={galleryText(locale, "资源状态", "Resource states")}
          >
            <div className="grid gap-3 sm:grid-cols-2">
              <UiResourceState
                description={galleryText(locale, "正在读取最新组件契约。", "Reading the latest component contract.")}
                size="sm"
                state="loading"
                title={galleryText(locale, "加载中", "Loading")}
                variant="card"
              />
              <UiResourceState
                impact={galleryText(locale, "当前快照未生成。", "The current snapshot was not generated.")}
                nextStep={galleryText(locale, "检查开发服务器后重试。", "Check the development server, then retry.")}
                primaryAction={{ label: galleryText(locale, "重试", "Retry"), onClick: () => undefined }}
                size="sm"
                state="error"
                title={galleryText(locale, "无法加载", "Unable to load")}
                variant="card"
              />
              <UiResourceState
                description={galleryText(locale, "还没有保存的视觉基线。", "No visual baseline has been saved yet.")}
                primaryAction={{ label: galleryText(locale, "创建基线", "Create baseline"), onClick: () => undefined }}
                size="sm"
                state="empty"
                title={galleryText(locale, "暂无记录", "No records")}
                variant="card"
              />
              <UiResourceState
                description={galleryText(locale, "核心状态已通过当前检查。", "Core states passed the current audit.")}
                size="sm"
                state="success"
                title={galleryText(locale, "契约一致", "Contract aligned")}
                variant="card"
              />
            </div>
          </GallerySection>

          <GallerySection
            className="xl:col-span-2"
            description={galleryText(locale, "Portal、焦点与 Escape 必须在真实浏览器里验证；这里提供统一触发入口。", "Portal, focus, and Escape behavior must be verified in a real browser; these controls expose the production paths.")}
            eyebrow="06 · OVERLAYS"
            title={galleryText(locale, "浮层与响应式检查", "Overlay & responsive checks")}
          >
            <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(280px,0.65fr)]">
              <div className="flex flex-wrap items-center gap-3">
                <UiButton
                  onClick={() => {
                    setDialogViewport("adaptiveMax");
                    setDialogOpen(true);
                  }}
                  tone="primary"
                  variant="solid"
                >
                  <Layers3 className="h-4 w-4" />{galleryText(locale, "打开标准弹窗", "Open standard dialog")}
                </UiButton>
                <UiButton
                  onClick={() => {
                    setDialogViewport("compact");
                    setDialogOpen(true);
                  }}
                  variant="surface"
                >
                  {galleryText(locale, "打开紧凑弹窗", "Open compact dialog")}
                </UiButton>
                <UiButton onClick={() => setPromptOpen(true)} variant="surface">
                  {galleryText(locale, "新建文件夹弹窗", "New folder prompt")}
                </UiButton>
                <UiButton
                  onClick={() => {
                    setDialogViewport("visualPreview");
                    setDialogOpen(true);
                  }}
                  variant="surface"
                >
                  {galleryText(locale, "视觉预览尺寸", "Visual preview size")}
                </UiButton>
                <UiButton
                  onClick={() => {
                    setDialogViewport("documentPreview");
                    setDialogOpen(true);
                  }}
                  variant="surface"
                >
                  {galleryText(locale, "文档预览尺寸", "Document preview size")}
                </UiButton>
                <UiTooltip label={galleryText(locale, "键盘聚焦应立即显示，Escape 后焦点归还", "Keyboard focus opens immediately; Escape restores focus")} placement="top">
                  <UiButton variant="surface"><MousePointer2 className="h-4 w-4" />{galleryText(locale, "聚焦或悬停", "Focus or hover")}</UiButton>
                </UiTooltip>
                <UiIconButton aria-label={galleryText(locale, "浮层动作示例", "Overlay action example")} tooltip={galleryText(locale, "浮层动作示例", "Overlay action example")} tooltipShortcut="⌘ K" variant="surface">
                  <Sparkles className="h-4 w-4" />
                </UiIconButton>
              </div>
              <ol className="space-y-1.5 text-xs leading-5 text-(--text-muted)">
                <li><strong className="text-(--text-default)">320px: </strong>{galleryText(locale, "无横向溢出，动作自动换行，弹窗保留 8px inset。", "No horizontal overflow; actions wrap and dialogs preserve an 8px inset.")}</li>
                <li><strong className="text-(--text-default)">{galleryText(locale, "三主题：", "Three themes: ")}</strong>{galleryText(locale, "层级、边框和选中态不能靠页面私有 shadow/z-index 修补。", "Hierarchy, borders, and selection must not rely on page-private shadows or z-index values.")}</li>
                <li><strong className="text-(--text-default)">{galleryText(locale, "键盘：", "Keyboard: ")}</strong>{galleryText(locale, "Tab 可达、焦点可见、Escape 关闭并归还焦点。", "Tab reaches controls, focus stays visible, and Escape closes then restores focus.")}</li>
                <li><strong className="text-(--text-default)">Reduced motion: </strong>{galleryText(locale, "加载仍可理解，动画停止后状态信息不丢失。", "Loading remains understandable and no state is lost when animation stops.")}</li>
              </ol>
            </div>
          </GallerySection>
        </div>
        <div className="mt-5">
          <FoundationCompleteness locale={locale} />
        </div>
        </>
        ) : null}

        {activeTab === "content" ? <ContentGallery locale={locale} /> : null}
        {activeTab === "interaction" ? <InteractionGallery locale={locale} /> : null}
        {activeTab === "workspace" ? <WorkspaceGallery locale={locale} /> : null}
        {activeTab === "coverage" ? <CoverageInventory locale={locale} /> : null}
      </div>

      {dialogOpen ? (
        <UiDialogPortal>
          <UiDialogBackdrop
            inset={isGalleryViewer(dialogViewport) ? "compact" : "default"}
            labelledBy="gallery-dialog-title"
            onClose={() => setDialogOpen(false)}
          >
            <UiDialogShell
              size={getGalleryDialogSize(dialogViewport)}
              viewport={dialogViewport}
            >
              <UiDialogHeader
                icon={<Layers3 className="h-4 w-4" />}
                onClose={() => setDialogOpen(false)}
                subtitle={galleryText(locale, "检查标题、正文、Footer、焦点圈与窄窗口边距。", "Inspect title, body, footer, focus ring, and narrow-window insets.")}
                title={getGalleryDialogTitle(dialogViewport, locale)}
                titleId="gallery-dialog-title"
              />
              <UiDialogBody scrollable>
                <div className="space-y-4">
                  <UiField label={galleryText(locale, "变更摘要", "Change summary")}>
                    <UiTextarea key={locale} defaultValue={galleryText(locale, "相同功能只保留一个公共视觉入口；页面负责业务组合，不覆盖阴影、层级、圆角或焦点样式。", "Equivalent features keep one shared visual entry; pages compose business behavior without overriding shadow, layer, radius, or focus styles.")} />
                  </UiField>
                  <UiPanel padding="md" variant="dashed">
                    <div className="flex items-start gap-3">
                      <CircleAlert className="mt-0.5 h-4 w-4 shrink-0 text-(--warning)" />
                      <p className="text-sm leading-6 text-(--text-default)">
                        {galleryText(locale, "这条较长中文信息用于验证自然换行，同时不制造第二套卡片层级。", "This deliberately long English message verifies natural wrapping without creating a second card hierarchy.")}
                      </p>
                    </div>
                  </UiPanel>
                </div>
              </UiDialogBody>
              <UiDialogFooter>
                <UiButton onClick={() => setDialogOpen(false)} variant="text">{galleryText(locale, "取消", "Cancel")}</UiButton>
                <UiButton onClick={() => setDialogOpen(false)} tone="primary" variant="solid"><Check className="h-4 w-4" />{galleryText(locale, "确认", "Confirm")}</UiButton>
              </UiDialogFooter>
            </UiDialogShell>
          </UiDialogBackdrop>
        </UiDialogPortal>
      ) : null}
      <PromptDialog
        cancelText={galleryText(locale, "取消", "Cancel")}
        confirmText={galleryText(locale, "创建", "Create")}
        defaultValue={galleryText(locale, "新文件夹", "new-folder")}
        isOpen={promptOpen}
        onCancel={() => setPromptOpen(false)}
        onConfirm={() => setPromptOpen(false)}
        placeholder={galleryText(locale, "例如：新文件夹", "For example: new-folder")}
        title={galleryText(locale, "新建文件夹", "New folder")}
      />
    </main>
  );
}

function GallerySection({
  children,
  className,
  description,
  eyebrow,
  title,
}: {
  children: ReactNode;
  className?: string;
  description: string;
  eyebrow: string;
  title: string;
}) {
  return (
    <section className={`surface-panel min-w-0 p-4 sm:p-5 ${className ?? ""}`} data-gallery-section={title}>
      <div className="mb-5 border-b border-(--divider-subtle-color) pb-4">
        <p className={getUiTypographyClassName({ role: "overline", tone: "brand" })}>{eyebrow}</p>
        <h2 className={cn(
          "mt-1",
          getUiTypographyClassName({ role: "objectTitle", tone: "strong" }),
        )}>{title}</h2>
        <p className={cn(
          "mt-1.5 max-w-2xl",
          getUiTypographyClassName({ role: "caption", tone: "muted" }),
        )}>{description}</p>
      </div>
      <div className="space-y-5">{children}</div>
    </section>
  );
}

function getGalleryDialogTitle(viewport: UiDialogViewport, locale: Locale): string {
  switch (viewport) {
    case "compact":
      return galleryText(locale, "紧凑弹窗契约", "Compact dialog contract");
    case "visualPreview":
      return galleryText(locale, "视觉预览契约", "Visual preview contract");
    case "documentPreview":
      return galleryText(locale, "文档预览契约", "Document preview contract");
    default:
      return galleryText(locale, "共享弹窗契约", "Shared dialog contract");
  }
}

function getGalleryDialogSize(viewport: UiDialogViewport): UiDialogSize {
  if (viewport === "visualPreview") {
    return "xl";
  }
  if (viewport === "documentPreview") {
    return "lg";
  }
  return "md";
}

function isGalleryViewer(viewport: UiDialogViewport): boolean {
  return viewport === "visualPreview" || viewport === "documentPreview";
}

function GalleryRow({ children, label }: { children: ReactNode; label: string }) {
  return (
    <div className="grid min-w-0 gap-2 sm:grid-cols-[112px_minmax(0,1fr)] sm:items-start">
      <span className={cn(
        "pt-1.5",
        getUiTypographyClassName({ role: "caption", tone: "soft", weight: "medium" }),
      )}>{label}</span>
      <div className="flex min-w-0 flex-wrap items-center gap-2">{children}</div>
    </div>
  );
}

function TypographySample({
  children,
  label,
  textRole,
  tone = "strong",
}: {
  children: ReactNode;
  label: string;
  textRole: UiTypographyRole;
  tone?: UiTypographyTone;
}) {
  return (
    <div className="min-w-0 border-l border-(--divider-subtle-color) pl-3" data-typography-role={textRole}>
      <span className={getUiTypographyClassName({ role: "caption", tone: "soft" })}>{label}</span>
      <p className={cn("mt-1 break-words", getUiTypographyClassName({ role: textRole, tone }))}>{children}</p>
    </div>
  );
}

function SurfaceSample({ label }: { label: string }) {
  return (
    <div className="flex min-h-12 items-center justify-between gap-2">
      <span className={getUiTypographyClassName({ role: "sectionTitle", tone: "default" })}>{label}</span>
      <Search className="h-3.5 w-3.5 text-(--icon-muted)" />
    </div>
  );
}
