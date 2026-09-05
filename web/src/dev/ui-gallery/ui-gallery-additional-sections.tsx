// INPUT: 当前 Gallery locale 与只用于演示交互的本地状态。
// OUTPUT: 直接渲染 shared/ui 真实组件的补全预览分组。
// POS: 开发期 Gallery 场景装配；只写 fixture，不复制产品组件实现。

import {
  Bot,
  Check,
  ChevronRight,
  CircleAlert,
  Ellipsis,
  FileCode2,
  Grid2X2,
  MoreHorizontal,
  Plus,
  RefreshCw,
  Search,
  Settings2,
  Sparkles,
  Trash2,
} from "lucide-react";
import {
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";

import type { Locale } from "@/shared/i18n/messages";
import { AgentOptionsGallery } from "./ui-gallery-agent-options";
import { UiButton, UiIconButton, UiLinkButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { UiDisclosure } from "@/shared/ui/disclosure/disclosure";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiMetaGrid, UiMetaItem } from "@/shared/ui/display/meta-grid";
import { UiQRCode } from "@/shared/ui/display/qr-code";
import { UiSeededAvatar } from "@/shared/ui/display/seeded-avatar";
import { UiSkeleton, UiSkeletonCardList } from "@/shared/ui/display/skeleton";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import { AnimatedHeroText, FadeSlideIn } from "@/shared/ui/feedback/animated-hero-text";
import { FeedbackBanner } from "@/shared/ui/feedback/feedback-banner";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { LoadingOrb } from "@/shared/ui/feedback/loading-orb";
import { LottiePlayer } from "@/shared/ui/feedback/lottie-player";
import { RecoverySummary } from "@/shared/ui/feedback/recovery-summary";
import { TypewriterFileView } from "@/shared/ui/feedback/typewriter-file-view";
import { UiCheckboxRow } from "@/shared/ui/form/checkbox-row";
import { SidebarSearchAction, SidebarSearchField } from "@/shared/ui/form/sidebar-search-field";
import { IconPicker } from "@/shared/ui/icon-picker/icon-picker";
import {
  IconPickerPopover,
  IconPickerTriggerLabel,
} from "@/shared/ui/icon-picker/icon-picker-popover";
import { AppLoadingState } from "@/shared/ui/layout/app-loading-screen";
import { PanelResizeHandle } from "@/shared/ui/layout/panel-resize-handle";
import { WorkspaceContentDetailHeader, WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { GlassMagnifier } from "@/shared/ui/liquid-glass/glass-magnifier";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import { CodeBlock } from "@/shared/ui/markdown/code/code-block";
import { StreamingCodeBlock } from "@/shared/ui/markdown/code/streaming-code-block";
import { UiMarkdownContent } from "@/shared/ui/markdown/markdown-content";
import { MermaidView } from "@/shared/ui/markdown/mermaid/mermaid-view";
import { WorkspaceFileButton } from "@/shared/ui/markdown/workspace/markdown-workspace-file-button";
import { MentionTargetPopover } from "@/shared/ui/mention/mention-target-popover";
import { UiActionMenu, UiActionMenuContent } from "@/shared/ui/menu/action-menu";
import { UiBreadcrumb } from "@/shared/ui/navigation/breadcrumb";
import { TourOverlayCard } from "@/shared/ui/onboarding/overlay/tour-overlay-card";
import { OnboardingTourOverlay } from "@/shared/ui/onboarding/overlay/tour-overlay";
import { SidebarEmptyGuide } from "@/shared/ui/sidebar/sidebar-empty-guide";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { WorkspaceCatalogAction, WorkspaceCatalogTextAction } from "@/shared/ui/workspace/catalog/workspace-catalog-actions";
import { WorkspaceCatalogCard, WorkspaceCatalogGhostAction } from "@/shared/ui/workspace/catalog/workspace-catalog-card";
import {
  WorkspaceCatalogBody,
  WorkspaceCatalogDescription,
  WorkspaceCatalogFooter,
  WorkspaceCatalogHeader,
  WorkspaceCatalogTitle,
} from "@/shared/ui/workspace/catalog/workspace-catalog-content";
import { WorkspaceIconFrame } from "@/shared/ui/workspace/catalog/workspace-icon-frame";
import { WorkspaceActionBar, WorkspaceActionCard } from "@/shared/ui/workspace/controls/workspace-action-bar";
import { ConversationTabsScrollRail } from "@/shared/ui/workspace/controls/conversation-tabs/conversation-tabs-scroll-rail";
import { WorkspaceConversationTab } from "@/shared/ui/workspace/controls/conversation-tabs/workspace-conversation-tab";
import { WorkspaceConversationTabs } from "@/shared/ui/workspace/controls/workspace-conversation-tabs";
import { WorkspaceSearchInput } from "@/shared/ui/workspace/controls/workspace-search-input";
import { WorkspaceStatusBadge } from "@/shared/ui/workspace/controls/workspace-status-badge";
import { WorkspaceLoadingState } from "@/shared/ui/workspace/frame/workspace-loading-state";
import { WorkspacePageFrame } from "@/shared/ui/workspace/frame/workspace-page-frame";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import { WorkspaceSurfaceView } from "@/shared/ui/workspace/surface/workspace-surface-view";
import { WorkspaceTaskPanel } from "@/shared/ui/workspace/surface/workspace-task-strip";
import { WorkspaceFileTree } from "@/shared/ui/workspace/tree/workspace-file-tree";
import type { WorkspaceFileEntry } from "@/types/agent/agent";
import type { TodoItem } from "@/types/conversation/todo";

import {
  UI_GALLERY_COMPONENT_NAMES,
  UI_GALLERY_COVERAGE_GROUPS,
  type UiGalleryCoverageMode,
} from "./ui-gallery-inventory";
import { galleryText } from "./ui-gallery-copy";

export function FoundationCompleteness({ locale }: { locale: Locale }) {
  const [checked, setChecked] = useState(true);
  const [sidebarSearch, setSidebarSearch] = useState("");

  return (
    <PreviewSection
      description={galleryText(locale, "补齐链接、带说明复选项、侧栏搜索、骨架、元数据、状态块、二维码和稳定种子头像。", "Completes links, described checkboxes, sidebar search, skeletons, metadata, state blocks, QR codes, and stable seeded avatars.")}
      eyebrow="07 · FOUNDATION"
      title={galleryText(locale, "基础组件补全", "Foundation completeness")}
    >
      <div className="grid gap-4 lg:grid-cols-2">
        <PreviewCard components={["UiLinkButton", "UiCheckboxRow", "UiDisclosure", "SidebarSearchField", "SidebarSearchAction"]}>
          <div className="flex flex-wrap gap-2">
            <UiLinkButton href="#coverage" tone="primary" variant="surface">
              {galleryText(locale, "跳到覆盖清单", "Open coverage index")} <ChevronRight className="h-4 w-4" />
            </UiLinkButton>
          </div>
          <UiCheckboxRow
            checked={checked}
            description={galleryText(locale, "整行点击仍由真实 checkbox 接收状态。", "The real checkbox still owns state when the whole row is clicked.")}
            label={galleryText(locale, "启用完整组件巡检", "Enable complete component audit")}
            onChange={setChecked}
          />
          <UiDisclosure
            label={galleryText(locale, "共享展开区域", "Shared disclosure")}
            meta={galleryText(locale, "2 项", "2 items")}
            variant="panel"
          >
            <p className={getUiTypographyClassName({ role: "caption", tone: "muted" })}>
              {galleryText(locale, "标题、箭头、焦点和正文边界由一个原语管理。", "One primitive owns the label, arrow, focus, and content boundary.")}
            </p>
          </UiDisclosure>
          <div className="rounded-[12px] border border-(--divider-subtle-color) py-2">
            <SidebarSearchField
              action={<SidebarSearchAction aria-label={galleryText(locale, "新建", "Create")}><Plus /></SidebarSearchAction>}
              onChange={setSidebarSearch}
              placeholder={galleryText(locale, "搜索组件", "Search components")}
              value={sidebarSearch}
            />
          </div>
        </PreviewCard>

        <PreviewCard components={["UiAgentAvatar", "UiRoomAvatar", "UiSeededAvatar", "UiMetaGrid", "UiMetaItem"]}>
          <div className="flex items-center gap-3">
            <UiAgentAvatar isWorking name="Nexus" size="md" />
            <UiSeededAvatar seed="nexus-ui-gallery" size="lg" />
            <UiSeededAvatar seed="nexus-ui-gallery-running" size="lg" state="running" />
            <div>
              <p className={getUiTypographyClassName({ role: "sectionTitle", tone: "strong" })}>Nexus UI</p>
              <p className={getUiTypographyClassName({ role: "caption", tone: "muted" })}>{galleryText(locale, "稳定种子头像", "Stable seeded avatar")}</p>
            </div>
          </div>
          <UiMetaGrid>
            <UiMetaItem label={galleryText(locale, "主题", "Theme")} value="Light / Dark / Rain" />
            <UiMetaItem label={galleryText(locale, "语言", "Locale")} value={locale === "zh" ? "简体中文" : "English"} />
            <UiMetaItem label={galleryText(locale, "来源", "Source")} value="shared/ui" />
          </UiMetaGrid>
        </PreviewCard>

        <PreviewCard components={["UiSkeleton", "UiSkeletonCardList"]}>
          <div className="space-y-2" aria-label={galleryText(locale, "骨架加载状态", "Skeleton loading state")}>
            <UiSkeleton className="h-4 w-2/5" />
            <UiSkeleton className="h-3 w-full" />
            <UiSkeleton className="h-3 w-4/5" />
          </div>
          <UiSkeletonCardList count={2} />
        </PreviewCard>

        <PreviewCard components={["UiStateBlock", "UiQRCode"]}>
          <div className="grid gap-3 sm:grid-cols-2">
            <UiStateBlock
              actions={<UiButton size="xs" variant="surface">{galleryText(locale, "查看", "Review")}</UiButton>}
              description={galleryText(locale, "用于只读状态和明确动作。", "For read-only status and explicit actions.")}
              icon={<CircleAlert className="h-4 w-4 text-(--warning)" />}
              size="sm"
              title={galleryText(locale, "需要检查", "Review required")}
              tone="warning"
              variant="card"
            />
            <div className="flex justify-center rounded-[12px] border border-(--divider-subtle-color) p-3">
              <UiQRCode
                alt={galleryText(locale, "组件预览地址二维码", "Component gallery URL QR code")}
                payload="http://localhost:3000/ui-gallery.html"
                showPayload
              />
            </div>
          </div>
        </PreviewCard>
      </div>
    </PreviewSection>
  );
}

export function ContentGallery({ locale }: { locale: Locale }) {
  const [showViewportBanner, setShowViewportBanner] = useState(false);
  const markdown = locale === "zh"
    ? "## 真实 Markdown 渲染\n\n支持 **强调**、列表和代码块。\n\n- 中英文自然换行\n- 代码与 Mermaid 复用同一入口\n\n```ts\nconst source = 'shared/ui';\n```\n\n```mermaid\nflowchart LR\n  Shared --> Gallery\n```"
    : "## Real Markdown rendering\n\nSupports **emphasis**, lists, and code blocks.\n\n- Natural bilingual wrapping\n- Code and Mermaid share one entry\n\n```ts\nconst source = 'shared/ui';\n```\n\n```mermaid\nflowchart LR\n  Shared --> Gallery\n```";
  const code = "type Coverage = 'direct' | 'composed';\nconst complete: Coverage = 'direct';";

  return (
    <div className="grid items-start gap-5 xl:grid-cols-2" data-gallery-panel="content">
      <PreviewSection
        description={galleryText(locale, "通知、恢复说明与固定视口都使用产品中的真实反馈组件。", "Notices, recovery guidance, and the fixed viewport all use production feedback components.")}
        eyebrow="01 · FEEDBACK"
        title={galleryText(locale, "反馈与恢复", "Feedback and recovery")}
      >
        <PreviewCard components={["FeedbackBanner", "FeedbackBannerViewport", "UiInlineNotice", "RecoverySummary"]}>
          <FeedbackBanner
            message={galleryText(locale, "组件清单已与 shared/ui 同步。", "The component inventory is synchronized with shared/ui.")}
            onDismiss={() => undefined}
            title={galleryText(locale, "同步完成", "Sync complete")}
            tone="success"
          />
          <FeedbackBanner
            action={{ label: galleryText(locale, "重新检查", "Check again"), onClick: () => undefined }}
            impact={galleryText(locale, "一个新组件尚未加入可视清单。", "A new component is not yet in the visual inventory.")}
            title={galleryText(locale, "覆盖不完整", "Coverage incomplete")}
            tone="warning"
          />
          <UiInlineNotice
            action={{
              icon: <RefreshCw />,
              label: galleryText(locale, "重新检查", "Check again"),
              onClick: () => undefined,
            }}
            icon={<CircleAlert />}
            message={galleryText(locale, "保留上一次成功读取的内容。", "The last successful content remains available.")}
            title={galleryText(locale, "资源暂时不可用", "Resource temporarily unavailable")}
            tone="warning"
          />
          <UiInlineNotice
            icon={<CircleAlert />}
            message={galleryText(locale, "工作图草稿缺少最终交付节点。", "The WorkGraph draft is missing its terminal delivery.")}
            tone="danger"
            width="compact"
          />
          <RecoverySummary
            impact={galleryText(locale, "当前页面仍可继续使用。", "The current page remains usable.")}
            nextStep={galleryText(locale, "补充预览后重新运行契约测试。", "Add its preview, then rerun the contract test.")}
          />
          <UiButton onClick={() => setShowViewportBanner(true)} size="sm" variant="surface">
            {galleryText(locale, "演示固定反馈视口", "Preview fixed feedback viewport")}
          </UiButton>
          <FeedbackBannerViewport
            item={showViewportBanner ? {
              impact: galleryText(locale, "此条由真实固定视口渲染。", "This message is rendered by the real fixed viewport."),
              onDismiss: () => setShowViewportBanner(false),
              title: galleryText(locale, "视口级反馈", "Viewport feedback"),
              tone: "warning",
            } : null}
          />
        </PreviewCard>
      </PreviewSection>

      <PreviewSection
        description={galleryText(locale, "动效组件在语言切换和 reduced-motion 下仍保留可读状态。", "Motion components retain readable state across locale changes and reduced motion.")}
        eyebrow="02 · MOTION"
        title={galleryText(locale, "加载与过渡", "Loading and transitions")}
      >
        <PreviewCard components={["AnimatedHeroText", "FadeSlideIn", "LoadingOrb", "LottiePlayer", "TypewriterFileView"]}>
          <div className="flex min-h-20 items-center justify-between gap-4 rounded-[12px] border border-(--divider-subtle-color) p-4">
            <div className="min-w-0">
              <AnimatedHeroText key={locale} text={galleryText(locale, "组件状态清晰可见", "Component states stay legible")} />
              <FadeSlideIn key={`fade-${locale}`}>
                <p className="mt-1 text-xs text-(--text-muted)">{galleryText(locale, "真实动效，不是静态替身。", "Real motion, not a static substitute.")}</p>
              </FadeSlideIn>
            </div>
            <div className="flex shrink-0 items-center gap-3">
              <LoadingOrb />
              <LoadingOrb variant="preparing" />
              <LottiePlayer className="h-12 w-12" src="/lotties/sparkles-icon-animation_IVlJtTDRww.lottie" />
            </div>
          </div>
          <TypewriterFileView
            content={galleryText(locale, "共享组件来自唯一实现。\n语言切换只改变 fixture 文案。", "Shared components come from one implementation.\nLocale changes only replace fixture copy.")}
            containerWidth={520}
          />
        </PreviewCard>
      </PreviewSection>

      <PreviewSection
        className="xl:col-span-2"
        description={galleryText(locale, "正文、代码、流式代码、Mermaid 和工作区文件按钮全部直接使用共享渲染器。", "Body copy, code, streaming code, Mermaid, and workspace file actions all use shared renderers directly.")}
        eyebrow="03 · CONTENT"
        title={galleryText(locale, "Markdown 与代码", "Markdown and code")}
      >
        <div className="grid gap-4 xl:grid-cols-2">
          <PreviewCard components={["UiMarkdownContent", "MarkdownText", "CodeBlock", "CodeBlockContent", "CodeShell", "SyntaxHighlightedCode"]}>
            <UiMarkdownContent content={markdown} onOpenWorkspaceFile={() => undefined} />
            <CodeBlock language="ts" value={code} />
            <WorkspaceFileButton
              label="web/src/shared/ui/markdown/markdown-content.tsx"
              onOpenWorkspaceFile={() => undefined}
              path="web/src/shared/ui/markdown/markdown-content.tsx"
            />
          </PreviewCard>
          <PreviewCard components={["StreamingCodeBlock", "MermaidView", "LazyMermaidView", "MermaidSourceView", "MermaidRenderedPreview", "MermaidPreviewDialog"]}>
            <StreamingCodeBlock language="ts" value={code} />
            <MermaidView
              chart="flowchart LR\n  Source[shared/ui] --> Gallery[UI Gallery]\n  Gallery --> Check[Coverage contract]"
              compact
              constrainHeight={false}
            />
          </PreviewCard>
        </div>
      </PreviewSection>

      <PreviewSection
        className="xl:col-span-2"
        description={galleryText(locale, "应用加载态与侧栏空态保持真实尺寸，但限制在预览容器内。", "App loading and sidebar empty states keep their real geometry inside bounded preview containers.")}
        eyebrow="04 · STATES"
        title={galleryText(locale, "应用与侧栏状态", "App and sidebar states")}
      >
        <div className="grid gap-4 lg:grid-cols-2">
          <PreviewCard components={["AppLoadingState", "AppLoadingScreen"]}>
            <AppLoadingState
              animationClassName="h-24 w-24"
              message={galleryText(locale, "正在加载组件目录…", "Loading component catalog…")}
            />
          </PreviewCard>
          <PreviewCard components={["SidebarEmptyGuide"]}>
            <SidebarEmptyGuide
              actionLabel={galleryText(locale, "创建组件", "Create component")}
              description={galleryText(locale, "创建后会出现在这个目录中。", "New components appear in this catalog.")}
              icon={Grid2X2}
              onAction={() => undefined}
              title={galleryText(locale, "暂无自定义组件", "No custom components")}
            />
          </PreviewCard>
        </div>
      </PreviewSection>
    </div>
  );
}

export function InteractionGallery({ locale }: { locale: Locale }) {
  const actionAnchorRef = useRef<HTMLButtonElement>(null);
  const mentionAnchorRef = useRef<HTMLButtonElement>(null);
  const [actionMenuOpen, setActionMenuOpen] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [glassChecked, setGlassChecked] = useState(true);
  const [icon, setIcon] = useState("agent-3");
  const [mentionRect, setMentionRect] = useState<DOMRect | null>(null);
  const [tourOpen, setTourOpen] = useState(false);
  const actionItems = useMemo(() => [
    { icon: <Check className="h-4 w-4" />, label: galleryText(locale, "设为当前", "Set as current"), value: "current", active: true },
    { label: galleryText(locale, "暂不可用", "Temporarily unavailable"), value: "unavailable", disabled: true },
    { icon: <Settings2 className="h-4 w-4" />, label: galleryText(locale, "编辑设置", "Edit settings"), value: "settings" },
  ], [locale]);
  const footerItems = useMemo(() => [
    { icon: <Trash2 className="h-4 w-4" />, label: galleryText(locale, "删除", "Delete"), value: "delete", tone: "danger" as const },
  ], [locale]);
  const tour = useMemo(() => ({
    id: "gallery-tour",
    steps: [{
      description: galleryText(locale, "引导层直接使用产品 Tour 卡片和导航动作。", "The guide uses the production Tour card and navigation actions."),
      id: "coverage",
      items: [
        { icon: "puzzle" as const, text: galleryText(locale, "逐项检查真实组件", "Inspect real components one by one") },
        { icon: "users" as const, text: galleryText(locale, "验证中英文长度", "Verify Chinese and English lengths") },
      ],
      placement: "center" as const,
      title: galleryText(locale, "组件预览导览", "Component gallery tour"),
    }],
  }), [locale]);

  return (
    <div className="grid items-start gap-5 xl:grid-cols-2" data-gallery-panel="interaction">
      <PreviewSection
        description={galleryText(locale, "菜单、Mention 与图标选择器都保留真实 Portal、键盘和焦点生命周期。", "Menus, mentions, and icon pickers retain their real portal, keyboard, and focus lifecycles.")}
        eyebrow="01 · POPOVERS"
        title={galleryText(locale, "菜单与选择器", "Menus and pickers")}
      >
        <PreviewCard components={["UiActionMenu", "UiActionMenuContent", "UiMenuActionRow"]}>
          <UiButton
            ref={actionAnchorRef}
            aria-expanded={actionMenuOpen}
            onClick={() => setActionMenuOpen((value) => !value)}
            variant="surface"
          >
            <Ellipsis className="h-4 w-4" /> {galleryText(locale, "打开动作菜单", "Open action menu")}
          </UiButton>
          <UiActionMenu
            anchorRef={actionAnchorRef}
            ariaLabel={galleryText(locale, "组件动作", "Component actions")}
            footerItems={footerItems}
            isOpen={actionMenuOpen}
            items={actionItems}
            onClose={() => setActionMenuOpen(false)}
            onSelect={() => undefined}
          />
          <div className="rounded-[12px] border border-(--divider-subtle-color) p-1">
            <UiActionMenuContent footerItems={footerItems} items={actionItems} onSelect={() => undefined} />
          </div>
        </PreviewCard>

        <PreviewCard components={[
          "IconPicker",
          "IconPickerPopover",
          "IconPickerTriggerLabel",
          "MentionTargetPopover",
        ]}>
          <IconPicker
            iconFamily="agent"
            layout="row"
            maxIcons={10}
            onSelect={setIcon}
            showClear
            value={icon}
          />
          <div className="flex flex-wrap gap-2">
            <IconPickerPopover
              ariaLabel={galleryText(locale, "选择 Agent 图标", "Choose Agent icon")}
              iconFamily="agent"
              maxIcons={10}
              onSelect={setIcon}
              renderTrigger={(isOpen) => (
                <IconPickerTriggerLabel isOpen={isOpen}>
                  {isOpen
                    ? galleryText(locale, "关闭图标库", "Close icon library")
                    : galleryText(locale, "打开图标库", "Open icon library")}
                </IconPickerTriggerLabel>
              )}
              startIconId={1}
              value={icon}
            />
            <UiButton
              ref={mentionAnchorRef}
              onClick={() => setMentionRect(mentionAnchorRef.current?.getBoundingClientRect() ?? null)}
              variant="surface"
            >
              @ {galleryText(locale, "选择成员", "Mention member")}
            </UiButton>
          </div>
          <MentionTargetPopover
            anchorRect={mentionRect}
            filter=""
            items={[
              { id: "maya", label: "Maya", marker: "M", subtitle: galleryText(locale, "前端", "Frontend") },
              { id: "lin", label: "Lin", marker: "L", subtitle: galleryText(locale, "设计系统", "Design system") },
            ]}
            onClose={() => setMentionRect(null)}
            onSelect={() => setMentionRect(null)}
          />
        </PreviewCard>
      </PreviewSection>

      <PreviewSection
        description={galleryText(locale, "玻璃开关和放大镜直接运行真实能力检测与交互 Hook。", "The glass switch and magnifier run their production capability checks and interaction hooks.")}
        eyebrow="02 · MATERIAL"
        title="Liquid glass"
      >
        <PreviewCard components={["GlassSwitch", "GlassSwitchFilter", "GlassMagnifier", "GlassMagnifierFilter"]}>
          <div className="flex min-h-28 items-center justify-around rounded-[16px] bg-[radial-gradient(circle_at_25%_25%,color-mix(in_srgb,var(--primary)_28%,transparent),transparent_42%),linear-gradient(135deg,var(--surface-panel-background),var(--surface-control-background))] p-5">
            <GlassSwitch
              aria-label={galleryText(locale, "启用玻璃效果", "Enable glass effect")}
              checked={glassChecked}
              onChange={setGlassChecked}
            />
            <GlassMagnifier
              underlay={<div className="h-full w-full bg-[linear-gradient(90deg,var(--primary),var(--success))]" />}
            >
              <Search className="h-4 w-4 text-white" />
            </GlassMagnifier>
          </div>
        </PreviewCard>
      </PreviewSection>

      <PreviewSection
        description={galleryText(locale, "确认框与导览层通过按钮打开，便于验证焦点、Escape 与窄屏几何。", "Open confirmation and tour overlays to verify focus, Escape, and narrow-screen geometry.")}
        eyebrow="03 · MODAL"
        title={galleryText(locale, "确认与引导", "Confirmation and onboarding")}
      >
        <PreviewCard components={["ConfirmDialog", "DecisionDialogFrame", "DecisionDialogActions"]}>
          <UiButton onClick={() => setConfirmOpen(true)} tone="danger" variant="surface">
            {galleryText(locale, "打开确认框", "Open confirmation")}
          </UiButton>
          <ConfirmDialog
            cancelText={galleryText(locale, "取消", "Cancel")}
            confirmText={galleryText(locale, "确认删除", "Delete")}
            isOpen={confirmOpen}
            message={galleryText(locale, "这只是组件预览，不会删除任何数据。", "This is only a component preview; no data will be deleted.")}
            onCancel={() => setConfirmOpen(false)}
            onConfirm={() => setConfirmOpen(false)}
            title={galleryText(locale, "删除预览记录？", "Delete preview record?")}
            variant="danger"
          />
        </PreviewCard>

        <PreviewCard components={["TourOverlayCard", "OnboardingTourOverlay", "OnboardingTourProvider"]}>
          <TourOverlayCard
            isLastStep={false}
            onClose={() => undefined}
            onNext={() => undefined}
            onPrevious={() => undefined}
            placement="bottom"
            step={tour.steps[0]}
            stepCount={3}
            stepIndex={0}
          />
          <UiButton onClick={() => setTourOpen(true)} variant="surface">
            {galleryText(locale, "全屏导览检查", "Preview full tour overlay")}
          </UiButton>
          {tourOpen ? (
            <OnboardingTourOverlay
              onClose={() => setTourOpen(false)}
              onNext={() => undefined}
              onPrevious={() => undefined}
              stepIndex={0}
              tour={tour}
            />
          ) : null}
        </PreviewCard>
      </PreviewSection>
    </div>
  );
}

const WORKSPACE_FILES: WorkspaceFileEntry[] = [
  { depth: 0, is_dir: true, modified_at: "2026-09-03T00:00:00Z", name: "shared", path: "web/src/shared" },
  { depth: 1, is_dir: true, modified_at: "2026-09-03T00:00:00Z", name: "ui", path: "web/src/shared/ui" },
  { depth: 2, is_dir: false, modified_at: "2026-09-03T00:00:00Z", name: "button.tsx", path: "web/src/shared/ui/button.tsx", size: 2048 },
  { depth: 2, is_dir: false, modified_at: "2026-09-03T00:00:00Z", name: "dialog.tsx", path: "web/src/shared/ui/dialog.tsx", size: 4096 },
];

const WORKSPACE_TODOS: TodoItem[] = [
  { content: "Inventory shared components", status: "completed" },
  { active_form: "Rendering real bilingual fixtures", content: "Render bilingual fixtures", status: "in_progress" },
  { content: "Verify 320px viewport", status: "pending" },
];

const WORKSPACE_TABS = [
  { id: "gallery-one", titleZh: "UI 覆盖", titleEn: "UI coverage", isPinned: false },
  { id: "gallery-two", titleZh: "响应式文案", titleEn: "Responsive copy", isPinned: false },
  { id: "gallery-three", titleZh: "键盘状态", titleEn: "Keyboard states", isPinned: false },
];

export function WorkspaceGallery({ locale }: { locale: Locale }) {
  const [catalogActions, setCatalogActions] = useState({ primary: 0, secondary: 0 });
  const [catalogCreations, setCatalogCreations] = useState(0);
  const [activeSurfaceTab, setActiveSurfaceTab] = useState("overview");
  const [conversationId, setConversationId] = useState("gallery-one");
  const [conversationTabs, setConversationTabs] = useState(WORKSPACE_TABS);
  const nextConversationNumber = useRef(4);
  const [standaloneTab, setStandaloneTab] = useState({ isOpen: true, isPinned: true, isActive: true });
  const [scrollLeft, setScrollLeft] = useState(96);
  const [query, setQuery] = useState("");

  return (
    <div className="grid items-start gap-5" data-gallery-panel="workspace">
      <AgentOptionsGallery locale={locale} />
      <PreviewSection
        description={galleryText(locale, "目录卡、内容区和动作全部来自 Workspace 公共原语。", "Catalog cards, content slots, and actions all come from Workspace primitives.")}
        eyebrow="01 · CATALOG"
        title={galleryText(locale, "Workspace 目录", "Workspace catalog")}
      >
        <div className="grid gap-4 lg:grid-cols-3">
          <PreviewCard components={["WorkspaceCatalogCard", "WorkspaceCatalogHeader", "WorkspaceCatalogBody", "WorkspaceCatalogFooter", "WorkspaceCatalogTitle", "WorkspaceCatalogDescription", "WorkspaceIconFrame", "WorkspaceCatalogAction", "WorkspaceCatalogTextAction"]}>
            <WorkspaceCatalogCard
              aria-label="Catalog action example"
              primaryAction={{
                label: "Open catalog item",
                onClick: () => setCatalogActions((current) => ({ ...current, primary: current.primary + 1 })),
              }}
              size="compact"
            >
              <WorkspaceCatalogHeader>
                <WorkspaceIconFrame tone="primary"><Bot className="h-5 w-5" /></WorkspaceIconFrame>
                <div className="min-w-0">
                  <WorkspaceCatalogTitle size="sm">UI Auditor</WorkspaceCatalogTitle>
                  <WorkspaceCatalogDescription lines={2}>{galleryText(locale, "检查真实组件的主题、语言和交互状态。", "Checks real component themes, locales, and interaction states.")}</WorkspaceCatalogDescription>
                </div>
              </WorkspaceCatalogHeader>
              <WorkspaceCatalogBody grow><UiBadge size="xs">shared/ui</UiBadge></WorkspaceCatalogBody>
              <WorkspaceCatalogFooter>
                <WorkspaceCatalogTextAction>{galleryText(locale, "详情", "Details")}</WorkspaceCatalogTextAction>
                <WorkspaceCatalogAction
                  aria-label="Catalog secondary action"
                  onClick={() => setCatalogActions((current) => ({ ...current, secondary: current.secondary + 1 }))}
                ><MoreHorizontal className="h-4 w-4" /></WorkspaceCatalogAction>
              </WorkspaceCatalogFooter>
            </WorkspaceCatalogCard>
            <output data-gallery-catalog-actions>{catalogActions.primary}:{catalogActions.secondary}</output>
          </PreviewCard>

          <PreviewCard components={["WorkspaceCatalogGhostAction"]}>
            <WorkspaceCatalogGhostAction aria-label="Create catalog item" onClick={() => setCatalogCreations((count) => count + 1)} size="compact">
              <Plus className="h-5 w-5 text-(--icon-muted)" />
              <span className="mt-2 text-sm font-medium">{galleryText(locale, "新增目录项", "Add catalog item")}</span>
            </WorkspaceCatalogGhostAction>
            <WorkspaceCatalogGhostAction aria-label="Disabled catalog creation" disabled size="compact">
              {galleryText(locale, "暂不可创建", "Creation unavailable")}
            </WorkspaceCatalogGhostAction>
            <output data-gallery-catalog-creations>{catalogCreations}</output>
          </PreviewCard>

          <PreviewCard components={["WorkspaceActionBar", "WorkspaceActionCard"]}>
            <WorkspaceActionBar variant="cards">
              <WorkspaceActionCard icon={<Sparkles className="h-5 w-5" />} onClick={() => undefined} title={galleryText(locale, "生成预览", "Generate preview")} />
              <WorkspaceActionCard description={galleryText(locale, "打开完整目录", "Open full catalog")} icon={<Grid2X2 className="h-5 w-5" />} onClick={() => undefined} title={galleryText(locale, "浏览", "Browse")} />
            </WorkspaceActionBar>
          </PreviewCard>
        </div>
      </PreviewSection>

      <PreviewSection
        description={galleryText(locale, "真实 Header、Surface、Page Frame 和拖拽入口在受限容器内组合。", "Real headers, surfaces, page frames, and resize handles are composed inside bounded containers.")}
        eyebrow="02 · LAYOUT"
        title={galleryText(locale, "Workspace 布局", "Workspace layout")}
      >
        <PreviewCard components={["UiBreadcrumb", "WorkspaceContentHeader", "WorkspaceContentDetailHeader", "WorkspacePageFrame"]}>
          <div className="rounded-[12px] border border-(--divider-subtle-color) px-2 py-1">
            <UiBreadcrumb
              ariaLabel={galleryText(locale, "文件位置", "File location")}
              density="compact"
              items={[
                { id: "agent", label: "Nexus" },
                { id: "source", label: "web/src/shared/ui" },
                {
                  id: "file",
                  label: galleryText(locale, "组件契约说明.md", "component-contract.md"),
                },
              ]}
            />
          </div>
          <WorkspacePageFrame contentPaddingClassName="p-3">
            <WorkspaceContentHeader
              actions={<UiButton size="sm" tone="primary" variant="surface">{galleryText(locale, "新建", "Create")}</UiButton>}
              description={galleryText(locale, "标题、说明和动作共享一条基线。", "Title, description, and actions share one baseline.")}
              title={galleryText(locale, "组件目录", "Component catalog")}
            />
            <WorkspaceContentDetailHeader>
              <UiBreadcrumb
                ariaLabel={galleryText(locale, "当前位置", "Current location")}
                items={[
                  {
                    href: "#coverage",
                    id: "catalog",
                    label: galleryText(locale, "组件目录", "Component catalog"),
                  },
                  {
                    id: "detail",
                    label: galleryText(locale, "组件详情", "Component detail"),
                  },
                ]}
              />
            </WorkspaceContentDetailHeader>
          </WorkspacePageFrame>
        </PreviewCard>

        <PreviewCard components={["WorkspaceSurfaceHeader", "UiIconButton"]}>
          <WorkspaceSurfaceHeader
            activeTab={activeSurfaceTab}
            compactTabsLabel={galleryText(locale, "切换视图", "Switch view")}
            leading={<UiSeededAvatar seed="workspace-gallery" size="md" />}
            leadingVariant="identity"
            onChangeTab={setActiveSurfaceTab}
            subtitle={galleryText(locale, "真实共享头部", "Real shared header")}
            tabs={[
              { icon: Grid2X2, key: "overview", label: galleryText(locale, "概览", "Overview") },
              { icon: FileCode2, key: "files", label: galleryText(locale, "文件", "Files") },
            ]}
            title="Nexus UI"
            trailing={(
              <UiIconButton
                aria-label={galleryText(locale, "更多", "More")}
                size="sm"
                variant="ghost"
              >
                <Ellipsis className="h-4 w-4" />
              </UiIconButton>
            )}
          />
        </PreviewCard>

        <div className="grid gap-4 lg:grid-cols-2">
          <PreviewCard components={["WorkspaceSurfaceView", "WorkspaceSurfaceScaffold"]}>
            <div className="flex h-56 flex-col overflow-hidden rounded-[12px] border border-(--divider-subtle-color)">
              <WorkspaceSurfaceView
                bodyClassName="min-h-0"
                header={{ action: <UiButton size="xs" variant="text">{galleryText(locale, "刷新", "Refresh")}</UiButton>, kind: "page" }}
                title={galleryText(locale, "预览工作面", "Preview surface")}
              >
                <UiMetaGrid>
                  <UiMetaItem label="Section" value={activeSurfaceTab} />
                  <UiMetaItem label="Locale" value={locale} />
                </UiMetaGrid>
              </WorkspaceSurfaceView>
            </div>
            <div className="flex h-28 flex-col overflow-hidden rounded-[12px] border border-(--divider-subtle-color)">
              <WorkspaceSurfaceScaffold
                bodyClassName="grid place-items-center p-3 text-xs text-(--text-muted)"
                header={<div className="border-b border-(--divider-subtle-color) px-3 py-2 text-xs font-medium">WorkspaceSurfaceScaffold</div>}
              >
                {galleryText(locale, "独立骨架正文", "Standalone scaffold body")}
              </WorkspaceSurfaceScaffold>
            </div>
          </PreviewCard>
          <PreviewCard components={["PanelResizeHandle"]}>
            <div className="relative flex h-56 overflow-hidden rounded-[12px] border border-(--divider-subtle-color)">
              <div className="grid flex-1 place-items-center text-xs text-(--text-muted)">{galleryText(locale, "主面板", "Main panel")}</div>
              <div className="relative w-2 bg-(--surface-panel-subtle-background)">
                <PanelResizeHandle ariaLabel={galleryText(locale, "调整面板宽度", "Resize panel")} onResizeStart={() => undefined} variant="overlay" />
              </div>
              <div className="grid w-1/3 place-items-center border-l border-(--divider-subtle-color) text-xs text-(--text-muted)">{galleryText(locale, "辅助面板", "Side panel")}</div>
            </div>
          </PreviewCard>
        </div>
      </PreviewSection>

      <PreviewSection
        description={galleryText(locale, "会话标签、状态、搜索与任务条使用真实控制器或视图组件。", "Conversation tabs, status, search, and task strip use their real controllers or views.")}
        eyebrow="03 · CONTROLS"
        title={galleryText(locale, "Workspace 控件", "Workspace controls")}
      >
        <div className="grid gap-4 lg:grid-cols-2">
          <PreviewCard components={["WorkspaceSearchInput", "WorkspaceStatusBadge"]}>
            <WorkspaceSearchInput onChange={setQuery} placeholder={galleryText(locale, "搜索工作区", "Search workspace")} value={query} />
            <div className="flex flex-wrap gap-2">
              <WorkspaceStatusBadge label={galleryText(locale, "运行中", "Running")} tone="running" />
              <WorkspaceStatusBadge icon={<Check className="h-3.5 w-3.5" />} label={galleryText(locale, "已完成", "Complete")} tone="success" />
            </div>
          </PreviewCard>
          <PreviewCard components={["WorkspaceTaskPanel"]}>
            <div className="flex min-h-28 items-end justify-center rounded-[12px] border border-(--divider-subtle-color) px-2">
              <WorkspaceTaskPanel
                source={{ agentId: "gallery", avatar: null, name: "Nexus" }}
                todos={WORKSPACE_TODOS.map((todo) => ({
                  ...todo,
                  active_form: todo.active_form ? galleryText(locale, "正在渲染真实双语 fixture", todo.active_form) : undefined,
                  content: galleryText(locale, translateTodo(todo.content), todo.content),
                }))}
              />
            </div>
          </PreviewCard>
        </div>

        <PreviewCard components={["WorkspaceConversationTabs", "WorkspaceConversationTab", "ConversationTabsScrollRail"]}>
          <div className="rounded-[12px] border border-(--divider-subtle-color) px-2">
            <WorkspaceConversationTabs
              activeConversationId={conversationId}
              tabs={conversationTabs.map((tab) => ({
                id: tab.id,
                canClose: conversationTabs.length > 1,
                canPin: true,
                isPinned: tab.isPinned,
                title: galleryText(locale, tab.titleZh, tab.titleEn),
              }))}
              onCreateConversation={() => {
                const number = nextConversationNumber.current++;
                const id = `gallery-${number}`;
                setConversationTabs((tabs) => [...tabs, {
                  id,
                  titleZh: `新会话 ${number}`,
                  titleEn: `New session ${number}`,
                  isPinned: false,
                }]);
                setConversationId(id);
              }}
              onSelectConversation={setConversationId}
              onCloseConversation={(id) => {
                const index = conversationTabs.findIndex((tab) => tab.id === id);
                const nextTabs = conversationTabs.filter((tab) => tab.id !== id);
                setConversationTabs(nextTabs);
                if (conversationId === id) {
                  setConversationId(nextTabs[index]?.id ?? nextTabs[index - 1]?.id ?? "");
                }
              }}
              onTogglePin={(id) => setConversationTabs((tabs) => tabs.map((tab) => (
                tab.id === id ? { ...tab, isPinned: !tab.isPinned } : tab
              )))}
            />
          </div>
          <div className="relative h-12 overflow-hidden rounded-[12px] border border-(--divider-subtle-color) px-2 pt-1">
            {standaloneTab.isOpen ? <WorkspaceConversationTab
              canClose
              canPin
              closeLabel={galleryText(locale, "关闭会话", "Close conversation")}
              conversationId="standalone"
              externalSessionLabel="IM"
              isActive={standaloneTab.isActive}
              isPinned={standaloneTab.isPinned}
              onClose={() => setStandaloneTab((tab) => ({ ...tab, isOpen: false }))}
              onSelect={() => setStandaloneTab((tab) => ({ ...tab, isActive: !tab.isActive }))}
              onTogglePin={() => setStandaloneTab((tab) => ({ ...tab, isPinned: !tab.isPinned }))}
              pinLabel={standaloneTab.isPinned ? galleryText(locale, "取消固定", "Unpin") : galleryText(locale, "固定", "Pin")}
              title={galleryText(locale, "独立标签视图", "Standalone tab view")}
            /> : <UiButton onClick={() => setStandaloneTab((tab) => ({ ...tab, isOpen: true }))} size="xs" variant="ghost">
              {galleryText(locale, "恢复标签示例", "Restore tab example")}
            </UiButton>}
            <ConversationTabsScrollRail
              ariaLabel={galleryText(locale, "会话滚动", "Conversation scroll")}
              metrics={{ clientWidth: 240, maxScrollLeft: 320, scrollLeft, scrollWidth: 560 }}
              onChange={setScrollLeft}
            />
          </div>
        </PreviewCard>
      </PreviewSection>

      <PreviewSection
        description={galleryText(locale, "文件树和加载态直接消费共享资源视图，不构造相似 DOM。", "The file tree and loading state consume shared resource views without look-alike DOM.")}
        eyebrow="04 · RESOURCES"
        title={galleryText(locale, "Workspace 资源", "Workspace resources")}
      >
        <div className="grid gap-4 lg:grid-cols-2">
          <PreviewCard components={["WorkspaceFileTree", "WorkspaceFileTreeRow"]}>
            <WorkspaceFileTree
              activePath="web/src/shared/ui/button.tsx"
              entries={WORKSPACE_FILES}
              focusedDirectoryPath="web/src/shared/ui"
              onClickDirectory={() => undefined}
              onClickFile={() => undefined}
              onContextMenu={() => undefined}
              onDeleteEntry={() => undefined}
              onRenameEntry={() => undefined}
            />
          </PreviewCard>
          <PreviewCard components={["WorkspaceLoadingState"]}>
            <div className="flex min-h-48">
              <WorkspaceLoadingState label={galleryText(locale, "正在读取工作区…", "Loading workspace…")} />
            </div>
          </PreviewCard>
        </div>
      </PreviewSection>
    </div>
  );
}

export function CoverageInventory({ locale }: { locale: Locale }) {
  const modeLabels: Record<UiGalleryCoverageMode, string> = {
    direct: galleryText(locale, "直接渲染", "Direct render"),
    composed: galleryText(locale, "由真实组合覆盖", "Covered by composition"),
    infrastructure: galleryText(locale, "基础设施路径覆盖", "Infrastructure path"),
  };

  return (
    <div id="coverage" data-gallery-panel="coverage">
      <PreviewSection
        description={galleryText(locale, `当前清单登记 ${UI_GALLERY_COMPONENT_NAMES.length} 个公开 React 组件；契约测试会在新增导出未登记时失败。`, `${UI_GALLERY_COMPONENT_NAMES.length} public React components are registered; the contract test fails when a new export is not inventoried.`)}
        eyebrow="INDEX · CONTRACT"
        title={galleryText(locale, "完整覆盖清单", "Complete coverage index")}
      >
        <div className="grid gap-3 lg:grid-cols-2">
          {UI_GALLERY_COVERAGE_GROUPS.map((group) => (
            <article className="rounded-[12px] border border-(--divider-subtle-color) p-3" key={group.id}>
              <div className="flex items-center justify-between gap-3">
                <h3 className={getUiTypographyClassName({ role: "sectionTitle", tone: "strong" })}>{group.renderer}</h3>
                <UiBadge size="xs" tone={group.mode === "direct" ? "success" : group.mode === "composed" ? "running" : "default"}>
                  {modeLabels[group.mode]}
                </UiBadge>
              </div>
              <div className="mt-2 flex flex-wrap gap-1.5">
                {group.components.map((component) => (
                  <code className="rounded-[6px] bg-(--surface-panel-subtle-background) px-1.5 py-1 text-[11px] text-(--text-default)" key={component}>
                    {component}
                  </code>
                ))}
              </div>
            </article>
          ))}
        </div>
      </PreviewSection>
    </div>
  );
}

function PreviewSection({
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
    <section className={cn("surface-panel min-w-0 p-4 sm:p-5", className)} data-gallery-section={title}>
      <div className="mb-5 border-b border-(--divider-subtle-color) pb-4">
        <p className={getUiTypographyClassName({ role: "overline", tone: "brand" })}>{eyebrow}</p>
        <h2 className={cn("mt-1", getUiTypographyClassName({ role: "objectTitle", tone: "strong" }))}>{title}</h2>
        <p className={cn("mt-1.5 max-w-3xl", getUiTypographyClassName({ role: "caption", tone: "muted" }))}>{description}</p>
      </div>
      <div className="space-y-4">{children}</div>
    </section>
  );
}

function PreviewCard({ children, components }: { children: ReactNode; components: readonly string[] }) {
  return (
    <article className="min-w-0 space-y-3 rounded-[14px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-panel-background)_72%,transparent)] p-3" data-gallery-components={components.join(",")}>
      <div className="flex flex-wrap gap-1.5 border-b border-(--divider-subtle-color) pb-2">
        {components.map((component) => (
          <code className="text-[10px] leading-4 text-(--text-soft)" key={component}>{component}</code>
        ))}
      </div>
      {children}
    </article>
  );
}

function translateTodo(content: string): string {
  const translations: Record<string, string> = {
    "Inventory shared components": "登记共享组件",
    "Render bilingual fixtures": "渲染双语示例",
    "Verify 320px viewport": "验证 320px 视口",
  };
  return translations[content] ?? content;
}
