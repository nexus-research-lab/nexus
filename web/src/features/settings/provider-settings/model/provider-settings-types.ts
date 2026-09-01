import type {
  ProviderApiFormat,
  ProviderKind,
  ProviderModelCapabilities,
  ProviderModelRecord,
} from "@/types/capability/provider";
import type { MutationFailureEffect } from "@/lib/error-message";

export type FormMode = "empty" | "create" | "edit";

interface FeedbackStateBase {
  mutationEffect?: MutationFailureEffect;
  recoveryAction?: "refresh";
  title: string;
}

export type FeedbackState =
  | FeedbackStateBase & {
    impact: string;
    message?: never;
    tone: "error" | "warning";
  }
  | FeedbackStateBase & {
    impact?: never;
    message: string;
    tone: "success";
  };

export interface ProviderDraft {
  provider_kind: ProviderKind;
  provider: string;
  preset_key: string;
  api_format: ProviderApiFormat;
  display_name: string;
  auth_token: string;
  base_url: string;
  models_path: string;
  enabled: boolean;
}

export interface ModelOptionsState {
  model: ProviderModelRecord;
  capabilities: ProviderModelCapabilities;
  context_window: string;
  max_output_tokens: string;
  provider_options_text: string;
}
