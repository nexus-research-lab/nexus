// INPUT: shared/ui 公开 primitive/pattern 与当前 theme、locale、viewport 状态。
// OUTPUT: 可交互、可固定 URL、可在真实浏览器截图的组件状态陈列面。
// POS: 开发期视觉验收工具；只消费公共 UI，不定义或导出产品组件。

import {
  Check,
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

const THEME_OPTIONS: Array<{ label: string; value: VisualTheme }> = [
  { label: "Light", value: "light" },
  { label: "Dark", value: "dark" },
  { label: "Rain", value: "rain" },
];

const LOCALE_OPTIONS: Array<{ label: string; value: Locale }> = [
  { label: "中文", value: "zh" },
  { label: "English", value: "en" },
];

const SELECT_OPTIONS = [
  { label: "默认对话模型", value: "default" },
  { label: "快速响应模型", value: "fast", badge: "推荐" },
  { label: "暂不可用的模型", value: "offline", disabled: true },
];

const ROOM_MEMBERS = [
  { id: "lin", name: "Lin" },
  { id: "maya", name: "Maya" },
  { id: "noah", name: "Noah" },
];

export function UiContractGallery() {
  const { locale, setLocale } = useI18n();
  const { theme, setTheme } = useTheme();
  const [activeTab, setActiveTab] = useState("overview");
  const [dialogOpen, setDialogOpen] = useState(false);
  const [dialogViewport, setDialogViewport] = useState<UiDialogViewport>("adaptiveMax");
  const [promptOpen, setPromptOpen] = useState(false);
  const [searchValue, setSearchValue] = useState("统一浮层间距");
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
    window.history.replaceState(null, "", `${window.location.pathname}?${query}`);
  }, [locale, visualTheme]);

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
            <h1 className="max-w-4xl text-2xl font-semibold tracking-[-0.04em] text-(--text-strong) sm:text-[32px]">
              Nexus UI Contract Gallery
            </h1>
            <p className="mt-2 max-w-3xl text-sm leading-6 text-(--text-muted)">
              这里展示真实公共组件，不使用页面级替身。修改 token、recipe 或 primitive 后，先在三主题和窄窗口下检查这里，再进入业务页面。
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-2 lg:justify-end">
            <UiSegmentedControl
              icon={Palette}
              onChange={setTheme}
              options={THEME_OPTIONS}
              title="Visual theme"
              value={visualTheme}
            />
            <UiSegmentedControl
              onChange={setLocale}
              options={LOCALE_OPTIONS}
              title="Interface locale"
              value={locale}
            />
          </div>
        </header>

        <div className="mb-6 flex min-w-0 items-center justify-between gap-4">
          <UiTabs
            activeValue={activeTab}
            ariaLabel="Gallery sections"
            onChange={setActiveTab}
            options={[
              { label: "Foundation", value: "overview" },
              { label: "Interaction", value: "interaction" },
              { label: "Responsive copy / 中英文长文案", value: "copy" },
            ]}
          />
          <span className="hidden text-xs text-(--text-soft) md:inline">
            Tab 聚焦 · Hover 检查 · Escape 关闭浮层
          </span>
        </div>

        <div className="grid items-start gap-5 xl:grid-cols-2">
          <GallerySection
            description="主动作不靠阴影表达；tone 只表达语义，variant 只表达表面层级。"
            eyebrow="01 · ACTIONS"
            title="Buttons & actions"
          >
            <GalleryRow label="Primary">
              <UiButton tone="primary" variant="solid"><Plus className="h-4 w-4" />新建会话</UiButton>
              <UiButton tone="primary" variant="surface">应用修改</UiButton>
              <UiButton tone="primary" variant="ghost">查看详情</UiButton>
              <UiButton tone="primary" variant="text">了解规范</UiButton>
            </GalleryRow>
            <GalleryRow label="Neutral / danger">
              <UiButton variant="surface">取消</UiButton>
              <UiButton variant="ghost"><Copy className="h-4 w-4" />复制</UiButton>
              <UiButton tone="danger" variant="surface"><Trash2 className="h-4 w-4" />删除</UiButton>
              <UiButton disabled>不可用</UiButton>
            </GalleryRow>
            <GalleryRow label="Busy / icon">
              <UiButton aria-busy disabled tone="primary" variant="solid">
                <LoaderCircle className="h-4 w-4 animate-spin motion-reduce:animate-none" />保存中
              </UiButton>
              <UiIconButton aria-label="刷新组件快照" tooltipShortcut="R" variant="surface"><RefreshCw className="h-4 w-4" /></UiIconButton>
              <UiIconButton aria-label="更多操作" tooltip="更多操作" variant="ghost"><Ellipsis className="h-4 w-4" /></UiIconButton>
              <UiCounterBadge count={108} />
            </GalleryRow>
          </GallerySection>

          <GallerySection
            description="输入壳、校验、布尔选择和互斥选择分别由稳定 primitive 持有。"
            eyebrow="02 · INPUT"
            title="Forms & selection"
          >
            <div className="grid gap-4 sm:grid-cols-2">
              <UiField description="辅助说明只出现一次。" htmlFor="gallery-name" label="名称">
                <UiInput defaultValue="前端基础治理" id="gallery-name" variant="surface" />
              </UiField>
              <UiField error="名称不能与现有配置重复。" htmlFor="gallery-error" label="错误状态">
                <UiInput aria-invalid defaultValue="重复名称" id="gallery-error" variant="surface" />
              </UiField>
            </div>
            <UiField label="搜索">
              <UiSearchInput onChange={setSearchValue} value={searchValue} />
            </UiField>
            <div className="grid gap-4 sm:grid-cols-3">
              <UiField label="模型">
                <UiSelectMenu
                  ariaLabel="选择模型"
                  onChange={setSelectedModel}
                  options={SELECT_OPTIONS}
                  value={selectedModel}
                />
              </UiField>
              <UiField label="备注">
                <UiTextarea defaultValue="长内容需要在 320px 下保持可读，不横向溢出。" variant="surface" />
              </UiField>
              <UiField htmlFor="gallery-native-role" label="原生角色">
                <UiNativeSelect
                  id="gallery-native-role"
                  onChange={(event) => setSelectedRole(event.target.value)}
                  value={selectedRole}
                  variant="surface"
                >
                  <option value="member">成员</option>
                  <option value="admin">管理员</option>
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
                <UiCheckbox defaultChecked id="gallery-remember" />记住选择
              </label>
            </GalleryRow>
          </GallerySection>

          <GallerySection
            description="身份、状态与选中态使用统一标记；列表动作不改变整行语义。"
            eyebrow="03 · DISPLAY"
            title="Identity & navigation"
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
                actions={<UiListActionButton aria-label="条目更多操作" visibility="visible"><Ellipsis className="h-4 w-4" /></UiListActionButton>}
                description="当前选中态不使用浮起阴影"
                leading={<UiAgentAvatar name="Lin" size="sm" />}
                meta={<UiBadge size="xs" tone="active">Active</UiBadge>}
                onClick={() => undefined}
                title="组件所有权检查"
              />
              <UiListRow
                description="Long English metadata stays readable without breaking the action column"
                leading={<UiAgentAvatar name="Noah" size="sm" />}
                onClick={() => undefined}
                title="Responsive content audit / 响应式文案检查"
              />
            </div>
            <div className="grid gap-3 sm:grid-cols-3">
              <UiPanel padding="sm" variant="card"><SurfaceSample label="card" /></UiPanel>
              <UiPanel padding="sm" variant="dashed"><SurfaceSample label="dashed" /></UiPanel>
              <UiPanel padding="sm" variant="plain"><SurfaceSample label="plain" /></UiPanel>
            </div>
          </GallerySection>

          <GallerySection
            description="加载、空、失败和完成是资源状态，不由页面临时拼接颜色与动作。"
            eyebrow="04 · FEEDBACK"
            title="Resource states"
          >
            <div className="grid gap-3 sm:grid-cols-2">
              <UiResourceState
                description="正在读取最新组件契约。"
                size="sm"
                state="loading"
                title="加载中"
                variant="card"
              />
              <UiResourceState
                impact="当前快照未生成。"
                nextStep="检查开发服务器后重试。"
                primaryAction={{ label: "重试", onClick: () => undefined }}
                size="sm"
                state="error"
                title="无法加载"
                variant="card"
              />
              <UiResourceState
                description="还没有保存的视觉基线。"
                primaryAction={{ label: "创建基线", onClick: () => undefined }}
                size="sm"
                state="empty"
                title="暂无记录"
                variant="card"
              />
              <UiResourceState
                description="核心状态已通过当前检查。"
                size="sm"
                state="success"
                title="契约一致"
                variant="card"
              />
            </div>
          </GallerySection>

          <GallerySection
            className="xl:col-span-2"
            description="Portal、焦点与 Escape 必须在真实浏览器里验证；这里提供统一触发入口。"
            eyebrow="05 · OVERLAYS"
            title="Overlay & responsive checks"
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
                  <Layers3 className="h-4 w-4" />打开标准弹窗
                </UiButton>
                <UiButton
                  onClick={() => {
                    setDialogViewport("compact");
                    setDialogOpen(true);
                  }}
                  variant="surface"
                >
                  打开紧凑弹窗
                </UiButton>
                <UiButton onClick={() => setPromptOpen(true)} variant="surface">
                  新建文件夹弹窗
                </UiButton>
                <UiButton
                  onClick={() => {
                    setDialogViewport("visualPreview");
                    setDialogOpen(true);
                  }}
                  variant="surface"
                >
                  视觉预览尺寸
                </UiButton>
                <UiButton
                  onClick={() => {
                    setDialogViewport("documentPreview");
                    setDialogOpen(true);
                  }}
                  variant="surface"
                >
                  文档预览尺寸
                </UiButton>
                <UiTooltip label="键盘聚焦应立即显示，Escape 后焦点归还" placement="top">
                  <UiButton variant="surface"><MousePointer2 className="h-4 w-4" />聚焦或悬停</UiButton>
                </UiTooltip>
                <UiIconButton aria-label="浮层动作示例" tooltip="浮层动作示例" tooltipShortcut="⌘ K" variant="surface">
                  <Sparkles className="h-4 w-4" />
                </UiIconButton>
              </div>
              <ol className="space-y-1.5 text-xs leading-5 text-(--text-muted)">
                <li><strong className="text-(--text-default)">320px：</strong>无横向溢出，动作自动换行，弹窗保留 8px inset。</li>
                <li><strong className="text-(--text-default)">三主题：</strong>层级、边框和选中态不能靠页面私有 shadow/z-index 修补。</li>
                <li><strong className="text-(--text-default)">键盘：</strong>Tab 可达、焦点可见、Escape 关闭并归还焦点。</li>
                <li><strong className="text-(--text-default)">Reduced motion：</strong>加载仍可理解，动画停止后状态信息不丢失。</li>
              </ol>
            </div>
          </GallerySection>
        </div>
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
                subtitle="检查标题、正文、Footer、焦点圈与窄窗口边距。"
                title={getGalleryDialogTitle(dialogViewport)}
                titleId="gallery-dialog-title"
              />
              <UiDialogBody scrollable>
                <div className="space-y-4">
                  <UiField label="变更摘要">
                    <UiTextarea defaultValue="相同功能只保留一个公共视觉入口；页面负责业务组合，不覆盖阴影、层级、圆角或焦点样式。" />
                  </UiField>
                  <UiPanel padding="md" variant="dashed">
                    <div className="flex items-start gap-3">
                      <CircleAlert className="mt-0.5 h-4 w-4 shrink-0 text-(--warning)" />
                      <p className="text-sm leading-6 text-(--text-default)">
                        This deliberately long bilingual message verifies wrapping without creating a second card hierarchy. 中英文混排必须自然换行。
                      </p>
                    </div>
                  </UiPanel>
                </div>
              </UiDialogBody>
              <UiDialogFooter>
                <UiButton onClick={() => setDialogOpen(false)} variant="text">取消</UiButton>
                <UiButton onClick={() => setDialogOpen(false)} tone="primary" variant="solid"><Check className="h-4 w-4" />确认</UiButton>
              </UiDialogFooter>
            </UiDialogShell>
          </UiDialogBackdrop>
        </UiDialogPortal>
      ) : null}
      <PromptDialog
        defaultValue="new-folder"
        isOpen={promptOpen}
        onCancel={() => setPromptOpen(false)}
        onConfirm={() => setPromptOpen(false)}
        placeholder="例如：new-folder"
        title="新建文件夹"
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
        <p className="text-2xs font-semibold tracking-[0.18em] text-(--brand-action)">{eyebrow}</p>
        <h2 className="mt-1 text-lg font-semibold tracking-[-0.025em] text-(--text-strong)">{title}</h2>
        <p className="mt-1.5 max-w-2xl text-xs leading-5 text-(--text-muted)">{description}</p>
      </div>
      <div className="space-y-5">{children}</div>
    </section>
  );
}

function getGalleryDialogTitle(viewport: UiDialogViewport): string {
  switch (viewport) {
    case "compact":
      return "紧凑弹窗契约";
    case "visualPreview":
      return "视觉预览契约";
    case "documentPreview":
      return "文档预览契约";
    default:
      return "共享弹窗契约";
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
      <span className="pt-1.5 text-xs font-medium text-(--text-soft)">{label}</span>
      <div className="flex min-w-0 flex-wrap items-center gap-2">{children}</div>
    </div>
  );
}

function SurfaceSample({ label }: { label: string }) {
  return (
    <div className="flex min-h-12 items-center justify-between gap-2">
      <span className="text-xs font-semibold text-(--text-default)">{label}</span>
      <Search className="h-3.5 w-3.5 text-(--icon-muted)" />
    </div>
  );
}
