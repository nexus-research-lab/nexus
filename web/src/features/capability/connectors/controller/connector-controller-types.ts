// INPUT: Connector 业务阶段确认的结果、影响、下一步和可选恢复动作。
// OUTPUT: 可由共享 FeedbackBanner 完整呈现的 Connector 反馈合同。
// POS: Connector 控制器与目录视图之间的反馈边界。
export interface ConnectorFeedback {
  action?: {
    label: string;
    onClick: () => void;
  };
  impact?: string;
  message?: string;
  nextStep?: string;
  persistent?: boolean;
  reconciliationConnectorId?: string;
  title: string;
  tone: "success" | "error" | "warning";
}

export type ReportConnectorFeedback = (feedback: ConnectorFeedback) => void;
