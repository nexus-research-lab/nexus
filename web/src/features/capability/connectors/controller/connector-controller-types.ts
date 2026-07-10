export interface ConnectorFeedback {
  message: string;
  title: string;
  tone: "success" | "error";
}

export type ReportConnectorFeedback = (feedback: ConnectorFeedback) => void;
