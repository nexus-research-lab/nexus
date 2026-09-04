// INPUT: shared/ui 的公开 React 组件名与组件预览中的真实渲染分组。
// OUTPUT: 可供 Gallery 和契约测试共同消费的完整覆盖清单。
// POS: 仅开发期组件目录；不参与产品构建或导出产品组件。

export type UiGalleryCoverageMode = "direct" | "composed" | "infrastructure";

export interface UiGalleryCoverageGroup {
  id: string;
  section: "foundation" | "content" | "interaction" | "workspace";
  mode: UiGalleryCoverageMode;
  renderer: string;
  components: readonly string[];
}

export const UI_GALLERY_COVERAGE_GROUPS: readonly UiGalleryCoverageGroup[] = [
  {
    id: "core-controls",
    section: "foundation",
    mode: "direct",
    renderer: "Core controls",
    components: [
      "UiButton", "UiLinkButton", "UiIconButton", "UiSplitButton", "UiCheckbox", "UiCheckboxRow",
      "UiChoiceButton", "UiField", "UiInput", "UiNativeSelect", "UiTextarea",
      "UiSearchInput", "UiSegmentedControl", "UiListRow", "UiListActionButton",
      "UiSelectMenu", "UiBreadcrumb", "UiTabs", "UiDirectoryTabs", "UiTooltip", "UiPanel", "SidebarSearchField",
      "SidebarSearchAction",
    ],
  },
  {
    id: "select-composition",
    section: "foundation",
    mode: "composed",
    renderer: "UiSelectMenu",
    components: [
      "SelectMenuView", "SelectMenuTriggerContent", "SelectMenuPanel", "SelectMenuOptionRow",
    ],
  },
  {
    id: "display",
    section: "foundation",
    mode: "direct",
    renderer: "Display primitives",
    components: [
      "UiAgentAvatar", "UiRoomAvatar", "UiSeededAvatar", "UiBadge", "UiCounterBadge",
      "UiResourceState", "UiStateBlock", "UiSkeleton", "UiSkeletonCardList",
      "UiMetaGrid", "UiMetaItem", "UiQRCode",
    ],
  },
  {
    id: "feedback",
    section: "content",
    mode: "direct",
    renderer: "Feedback and motion",
    components: [
      "FeedbackBanner", "FeedbackBannerViewport", "UiInlineNotice", "RecoverySummary", "LoadingOrb",
      "AnimatedHeroText", "FadeSlideIn", "TypewriterFileView", "LottiePlayer",
      "AppLoadingState", "SidebarEmptyGuide",
    ],
  },
  {
    id: "app-loading-screen",
    section: "content",
    mode: "composed",
    renderer: "AppLoadingState",
    components: ["AppLoadingScreen"],
  },
  {
    id: "markdown",
    section: "content",
    mode: "direct",
    renderer: "Markdown and code",
    components: [
      "UiMarkdownContent", "MarkdownText", "CodeBlock", "StreamingCodeBlock", "CodeShell",
      "SyntaxHighlightedCode", "CodeBlockContent", "MermaidView", "LazyMermaidView",
      "MermaidModeButton", "MermaidSourceView", "MermaidRenderedPreview",
      "MermaidPreviewDialog", "WorkspaceFileButton",
    ],
  },
  {
    id: "dialogs",
    section: "interaction",
    mode: "direct",
    renderer: "Dialogs",
    components: ["ConfirmDialog", "PromptDialog"],
  },
  {
    id: "dialog-primitives",
    section: "interaction",
    mode: "composed",
    renderer: "Dialogs",
    components: [
      "UiDialogPortal", "UiDialogBackdrop", "UiDialogShell", "UiDialogFormShell",
      "UiDialogHeader", "UiDialogBody", "UiDialogFooter", "UiDialogCloseButton",
      "DecisionDialogFrame", "DecisionDialogActions",
    ],
  },
  {
    id: "menus-and-pickers",
    section: "interaction",
    mode: "direct",
    renderer: "Menus and pickers",
    components: [
      "UiActionMenu", "UiActionMenuContent", "MentionTargetPopover", "IconPicker",
      "IconPickerPopover",
    ],
  },
  {
    id: "liquid-glass",
    section: "interaction",
    mode: "direct",
    renderer: "Liquid glass",
    components: ["GlassSwitch", "GlassMagnifier"],
  },
  {
    id: "liquid-glass-filters",
    section: "interaction",
    mode: "composed",
    renderer: "GlassSwitch / GlassMagnifier",
    components: ["GlassSwitchFilter", "GlassMagnifierFilter"],
  },
  {
    id: "onboarding",
    section: "interaction",
    mode: "direct",
    renderer: "Onboarding tour",
    components: ["OnboardingTourOverlay", "TourOverlayCard"],
  },
  {
    id: "onboarding-provider",
    section: "interaction",
    mode: "infrastructure",
    renderer: "OnboardingTourOverlay",
    components: ["OnboardingTourProvider"],
  },
  {
    id: "workspace-catalog",
    section: "workspace",
    mode: "direct",
    renderer: "Workspace catalog",
    components: [
      "WorkspaceCatalogAction", "WorkspaceCatalogTextAction", "WorkspaceCatalogCard",
      "WorkspaceCatalogGhostAction", "WorkspaceCatalogHeader", "WorkspaceCatalogBody",
      "WorkspaceCatalogFooter", "WorkspaceCatalogTitle", "WorkspaceCatalogDescription",
      "WorkspaceIconFrame",
    ],
  },
  {
    id: "workspace-layout",
    section: "workspace",
    mode: "direct",
    renderer: "Workspace layout",
    components: [
      "WorkspaceContentHeader", "WorkspaceContentDetailHeader", "WorkspacePageFrame",
      "WorkspaceSurfaceHeader", "WorkspaceSurfaceScaffold", "WorkspaceSurfaceView",
      "PanelResizeHandle",
    ],
  },
  {
    id: "workspace-controls",
    section: "workspace",
    mode: "direct",
    renderer: "Workspace controls",
    components: [
      "WorkspaceActionBar", "WorkspaceActionCard", "WorkspaceSearchInput",
      "WorkspaceStatusBadge", "WorkspaceConversationTabs", "WorkspaceConversationTab",
      "ConversationTabsScrollRail", "WorkspaceTaskPanel",
    ],
  },
  {
    id: "workspace-resource",
    section: "workspace",
    mode: "direct",
    renderer: "Workspace resources",
    components: ["WorkspaceFileTree", "WorkspaceFileTreeRow", "WorkspaceLoadingState"],
  },
] as const;

export const UI_GALLERY_COMPONENT_NAMES = UI_GALLERY_COVERAGE_GROUPS
  .flatMap((group) => group.components)
  .slice()
  .sort();
