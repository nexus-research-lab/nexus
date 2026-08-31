import { createRoot } from "react-dom/client";
import type { ReactNode } from "react";
import { RefreshCw } from "lucide-react";

import "@/app/globals.css";
import "./review.css";

import { ConversationReliabilityNotice } from "@/features/conversation/shared/conversation-reliability-notice";
import { ReadResourceReliabilityNotice } from "@/features/conversation/shared/read-resource-reliability-notice";
import { TextFileEditorReliability } from "@/features/conversation/shared/editor/text/text-file-editor-reliability";
import { ProviderSetupFailureView } from "@/features/onboarding/provider-setup/provider-setup-failure";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { ThemeProvider } from "@/shared/theme/theme-provider";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { FeedbackBanner } from "@/shared/ui/feedback/feedback-banner";

const noop = () => undefined;

type Story = {
  render: () => ReactNode;
  title: string;
};

const STORIES = {
  "feedback-not-applied": {
    title: "修改没有生效",
    render: () => (
      <FeedbackSurface>
        <FeedbackBanner
          action={{ label: "返回修改", onClick: noop }}
          impact="原有设置未变。"
          message="填写内容未通过检查。"
          nextStep="修正标记内容后再保存。"
          title="设置没有保存"
          tone="error"
        />
      </FeedbackSurface>
    ),
  },
  "feedback-accepted": {
    title: "请求已接收",
    render: () => (
      <FeedbackSurface>
        <FeedbackBanner
          impact="已有内容不受影响。"
          message="请求已经收到。"
          nextStep="可以离开此页，稍后查看进度。"
          title="任务正在处理"
          tone="info"
        />
      </FeedbackSurface>
    ),
  },
  "feedback-committed-refresh": {
    title: "已保存、刷新失败",
    render: () => (
      <FeedbackSurface>
        <FeedbackBanner
          action={{ label: "重新加载", onClick: noop }}
          impact="服务端内容已更新。"
          message="页面没能刷新。"
          nextStep="重新加载即可，请勿再次提交。"
          title="更改已保存"
          tone="warning"
        />
      </FeedbackSurface>
    ),
  },
  "feedback-outcome-unknown": {
    title: "结果尚未确认",
    render: () => (
      <FeedbackSurface>
        <FeedbackBanner
          action={{ label: "检查状态", onClick: noop }}
          impact="当前页面仍保留原设置，保存结果未核实。"
          message="返回时连接中断。"
          nextStep="先检查最新状态，不要重复保存。"
          title="保存结果待确认"
          tone="warning"
        />
      </FeedbackSurface>
    ),
  },
  "resource-load-failed": {
    title: "整块内容不可用",
    render: () => (
      <UiResourceState
        className="min-h-[260px] w-full"
        description="没有拿到最新列表。"
        impact="已保存任务没有变化。"
        nextStep="重新加载后再查看。"
        primaryAction={{
          icon: <RefreshCw className="h-3.5 w-3.5" />,
          label: "重新加载",
          onClick: noop,
        }}
        state="error"
        title="任务暂时无法加载"
        variant="card"
      />
    ),
  },
  "resource-stale-snapshot": {
    title: "保留旧快照",
    render: () => (
      <div className="review-list-panel">
        <ReadResourceReliabilityNotice
          impact="下方是上次加载的内容；这次读取没有修改任务。"
          isRefreshing={false}
          nextStep="需要时重新检查。"
          onRefresh={noop}
          problem="显示的是上次内容"
          resource="review-task-list"
          stale
        />
        <SyntheticRows />
      </div>
    ),
  },
  "conversation-delivery-unknown": {
    title: "消息投递未知",
    render: () => (
      <div className="review-conversation">
        <div className="review-message">请整理一下今天的项目进度。</div>
        <ConversationReliabilityNotice
          compact
          reliability={{
            failure: {
              code: "delivery_unknown",
              session_key: "review-session",
            },
            provider_retry: null,
            transport_phase: "healthy",
          }}
        />
        <div className="review-composer">继续输入消息…</div>
      </div>
    ),
  },
  "editor-conflict": {
    title: "编辑冲突",
    render: () => (
      <EditorSurface>
        <TextFileEditorReliability
          hasLoadedContent
          isLoading={false}
          isReconciling={false}
          isSaving={false}
          onAdoptLatest={noop}
          onLoadLatest={noop}
          onOverwrite={noop}
          onReconcile={noop}
          onRetrySave={noop}
          resourceFailure={null}
          revisionReady
          saveIssue={{ kind: "conflict", phase: "review" }}
        />
      </EditorSurface>
    ),
  },
  "editor-outcome-unknown": {
    title: "编辑保存结果未知",
    render: () => (
      <EditorSurface>
        <TextFileEditorReliability
          hasLoadedContent
          isLoading={false}
          isReconciling={false}
          isSaving={false}
          onAdoptLatest={noop}
          onLoadLatest={noop}
          onOverwrite={noop}
          onReconcile={noop}
          onRetrySave={noop}
          resourceFailure={null}
          revisionReady
          saveIssue={{
            attemptedDraft: "合成审阅草稿",
            expectedRevision: "review-revision",
            kind: "outcome_unknown",
            reconciliationFailed: false,
          }}
        />
      </EditorSurface>
    ),
  },
  "provider-persist-unknown": {
    title: "Provider 保存结果未知",
    render: () => (
      <div className="review-provider-card">
        <p className="review-provider-eyebrow">连接模型服务 · 保存配置</p>
        <h2>连接 OpenAI 兼容服务</h2>
        <div className="review-provider-field">API Key　••••••••••••••••</div>
        <ProviderFailureReview />
      </div>
    ),
  },
  "destructive-outcome-unknown": {
    title: "破坏性操作结果未知",
    render: () => (
      <>
        <SyntheticRows />
        <ConfirmDialog
          confirmText="核对任务列表"
          failure={{
            impact: "当前列表尚未核对，服务端状态待确认。",
            nextStep: "先返回列表核对，不要再次删除。",
            title: "删除结果待确认",
            tone: "warning",
          }}
          isOpen
          message="删除后，这个任务将停止后续运行。"
          onCancel={noop}
          onConfirm={noop}
          subtitle="每周项目汇总"
          title="删除定时任务？"
          variant="default"
        />
      </>
    ),
  },
} satisfies Record<string, Story>;

export const FAILURE_REVIEW_STORY_IDS = Object.keys(STORIES);

function SyntheticRows() {
  return (
    <div className="review-rows" aria-hidden="true">
      <div><span>项目周报整理</span><small>今天 18:00</small></div>
      <div><span>客户反馈归档</span><small>明天 09:30</small></div>
      <div><span>文档一致性检查</span><small>每周五</small></div>
    </div>
  );
}

function FeedbackSurface({ children }: { children: ReactNode }) {
  return (
    <div className="review-feedback-surface">
      <div className="review-feedback-toolbar">
        <span>工作区设置</span>
        <small>已自动保存</small>
      </div>
      <div className="review-feedback-content" aria-hidden="true">
        <div><span>默认工作区</span><small>产品研发</small></div>
        <div><span>任务通知</span><small>开启</small></div>
      </div>
      <div className="review-feedback-slot">{children}</div>
    </div>
  );
}

function EditorSurface({ children }: { children: ReactNode }) {
  return (
    <div className="review-editor">
      <div className="review-editor-toolbar"><span>产品说明.md</span><small>正在编辑</small></div>
      <div className="review-editor-copy">这里是用于视觉验收的合成草稿内容。用户已经输入的文字应当始终保留。</div>
      {children}
    </div>
  );
}

function ProviderFailureReview() {
  return (
    <ProviderSetupFailureView
      impact="原配置仍在，其他配置未被覆盖。"
      message="配置保存结果待确认"
      nextStep="先刷新列表，确认后再保存。"
      tone="warning"
    />
  );
}

function ReviewApp() {
  const requested = new URLSearchParams(window.location.search).get("story") ?? "feedback-not-applied";
  const story = STORIES[requested as keyof typeof STORIES] ?? STORIES["feedback-not-applied"];
  return (
    <ThemeProvider>
      <I18nProvider>
        <main className="review-page" data-review-ready="true" data-review-story={requested}>
          <section aria-label={story.title} className="review-stage">{story.render()}</section>
        </main>
      </I18nProvider>
    </ThemeProvider>
  );
}

createRoot(document.getElementById("root")!).render(<ReviewApp />);
