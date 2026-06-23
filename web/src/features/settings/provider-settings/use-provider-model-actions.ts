import { useCallback, useMemo, useState } from "react";
import type { Dispatch, SetStateAction } from "react";

import {
  fetch_provider_models_api,
  test_provider_config_api,
  test_provider_model_api,
  update_provider_model_api,
} from "@/lib/api/provider-config-api";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type {
  ProviderApiFormat,
  ProviderConfigRecord,
  ProviderModelRecord,
} from "@/types/capability/provider";

import {
  AUTO_TEST_MODEL_VALUE,
  FeedbackState,
  ModelOptionsState,
  build_test_model_options,
  filter_provider_models,
  model_options_from_record,
  model_update_payload,
  parse_provider_options,
  sort_models_enabled_first,
} from "./provider-settings-model";

type SaveProviderConfig = (options?: {
  show_error?: boolean;
  show_success?: boolean;
}) => Promise<ProviderConfigRecord | null>;

interface UseProviderModelActionsOptions {
  api_format: ProviderApiFormat;
  pending_action: string | null;
  selected_can_manage: boolean;
  selected_record: ProviderConfigRecord | null;
  set_feedback: Dispatch<SetStateAction<FeedbackState | null>>;
  set_pending_action: Dispatch<SetStateAction<string | null>>;
  save_provider: SaveProviderConfig;
  refresh_all: (preferred_provider?: string | null) => Promise<void>;
  t: I18nContextValue["t"];
}

export function useProviderModelActions({
  api_format,
  pending_action,
  selected_can_manage,
  selected_record,
  set_feedback,
  set_pending_action,
  save_provider,
  refresh_all,
  t,
}: UseProviderModelActionsOptions) {
  const [model_query, set_model_query] = useState("");
  const [model_options, set_model_options] =
    useState<ModelOptionsState | null>(null);
  const [add_model_open, set_add_model_open] = useState(false);
  const [manual_model_id, set_manual_model_id] = useState("");
  const [manual_model_enabled, set_manual_model_enabled] = useState(true);

  const filtered_models = useMemo(() => {
    return filter_provider_models(selected_record?.models ?? [], model_query);
  }, [model_query, selected_record]);
  const displayed_models = useMemo(
    () => sort_models_enabled_first(filtered_models),
    [filtered_models],
  );
  const test_model_options = useMemo(() => {
    return build_test_model_options(
      selected_record?.models ?? [],
      t("settings.providers.auto_select_model"),
    );
  }, [selected_record, t]);
  const manual_model_placeholder =
    selected_record?.models[0]?.model_id ||
    (api_format === "anthropic_messages" ? "opus-4.7" : "model-id");

  const reset_model_controls = useCallback(() => {
    set_model_query("");
    set_add_model_open(false);
    set_model_options(null);
    set_manual_model_id("");
    set_manual_model_enabled(true);
  }, []);

  const handle_fetch_models = useCallback(async () => {
    if (!selected_record || pending_action || !selected_can_manage) {
      return;
    }
    try {
      set_pending_action("fetch");
      const provider_record = await save_provider({
        show_error: true,
        show_success: false,
      });
      if (!provider_record) {
        return;
      }
      const result = await fetch_provider_models_api(provider_record.provider);
      await refresh_all(provider_record.provider);
      set_feedback({
        tone: "success",
        title: t("settings.providers.models_synced_title"),
        message: t("settings.providers.models_synced_message", {
          count: result.count,
        }),
      });
    } catch (error) {
      set_feedback({
        tone: "error",
        title: t("settings.providers.models_sync_failed_title"),
        message:
          error instanceof Error
            ? error.message
            : t("settings.providers.models_sync_failed_message"),
      });
    } finally {
      set_pending_action(null);
    }
  }, [
    pending_action,
    refresh_all,
    save_provider,
    selected_can_manage,
    selected_record,
    set_feedback,
    set_pending_action,
    t,
  ]);

  const handle_open_add_model = useCallback(() => {
    if (!selected_can_manage) {
      return;
    }
    set_manual_model_id("");
    set_manual_model_enabled(true);
    set_add_model_open(true);
  }, [selected_can_manage]);

  const handle_add_model = useCallback(async () => {
    if (!selected_record || pending_action || !selected_can_manage) {
      return;
    }
    const model_id = manual_model_id.trim();
    if (!model_id) {
      set_feedback({
        tone: "error",
        title: t("settings.providers.model_id_required_title"),
        message: t("settings.providers.model_id_required_message"),
      });
      return;
    }
    try {
      set_pending_action(`add-model:${model_id}`);
      await update_provider_model_api(selected_record.provider, model_id, {
        enabled: manual_model_enabled,
        is_default: false,
        capabilities_override: {},
        context_window: null,
        max_output_tokens: null,
        provider_options: {},
      });
      set_add_model_open(false);
      set_manual_model_id("");
      await refresh_all(selected_record.provider);
      set_feedback({
        tone: "success",
        title: t("settings.providers.model_added_title"),
        message: t("settings.providers.model_added_message", {
          model: model_id,
        }),
      });
    } catch (error) {
      set_feedback({
        tone: "error",
        title: t("settings.providers.model_add_failed_title"),
        message:
          error instanceof Error
            ? error.message
            : t("settings.providers.model_add_failed_message"),
      });
    } finally {
      set_pending_action(null);
    }
  }, [
    manual_model_enabled,
    manual_model_id,
    pending_action,
    refresh_all,
    selected_can_manage,
    selected_record,
    set_feedback,
    set_pending_action,
    t,
  ]);

  const handle_test_provider = useCallback(async () => {
    if (!selected_record || pending_action || !selected_can_manage) {
      return;
    }
    try {
      set_pending_action("test");
      const provider_record = await save_provider({
        show_error: true,
        show_success: false,
      });
      if (!provider_record) {
        return;
      }
      const result = await test_provider_config_api(provider_record.provider);
      await refresh_all(provider_record.provider);
      set_feedback({
        tone: result.success ? "success" : "error",
        title: result.success
          ? t("settings.providers.provider_test_passed_title")
          : t("settings.providers.provider_test_failed_title"),
        message: result.success
          ? t("settings.providers.test_model_message", {
              model: result.model || t("settings.providers.auto_model"),
            })
          : result.error || t("settings.providers.connectivity_failed"),
      });
    } catch (error) {
      set_feedback({
        tone: "error",
        title: t("settings.providers.provider_test_failed_title"),
        message:
          error instanceof Error
            ? error.message
            : t("settings.providers.check_network_auth"),
      });
    } finally {
      set_pending_action(null);
    }
  }, [
    pending_action,
    refresh_all,
    save_provider,
    selected_can_manage,
    selected_record,
    set_feedback,
    set_pending_action,
    t,
  ]);

  const handle_test_model = useCallback(
    async (model_id: string) => {
      if (!selected_record || pending_action || !selected_can_manage) {
        return;
      }
      const normalized_model_id = model_id.trim();
      if (!normalized_model_id) {
        return;
      }
      try {
        set_pending_action(`test:${normalized_model_id}`);
        const provider_record = await save_provider({
          show_error: true,
          show_success: false,
        });
        if (!provider_record) {
          return;
        }
        const result = await test_provider_model_api(
          provider_record.provider,
          normalized_model_id,
        );
        await refresh_all(provider_record.provider);
        set_feedback({
          tone: result.success ? "success" : "error",
          title: result.success
            ? t("settings.providers.model_test_passed_title")
            : t("settings.providers.model_test_failed_title"),
          message: result.success
            ? t("settings.providers.test_model_message", {
                model: result.model || normalized_model_id,
              })
            : result.error || t("settings.providers.connectivity_failed"),
        });
      } catch (error) {
        set_feedback({
          tone: "error",
          title: t("settings.providers.model_test_failed_title"),
          message:
            error instanceof Error
              ? error.message
              : t("settings.providers.check_network_auth_model"),
        });
      } finally {
        set_pending_action(null);
      }
    },
    [
      pending_action,
      refresh_all,
      save_provider,
      selected_can_manage,
      selected_record,
      set_feedback,
      set_pending_action,
      t,
    ],
  );

  const handle_test_selection = useCallback(
    (value: string) => {
      if (value === AUTO_TEST_MODEL_VALUE) {
        void handle_test_provider();
        return;
      }
      void handle_test_model(value);
    },
    [handle_test_model, handle_test_provider],
  );

  const handle_toggle_model = useCallback(
    async (model: ProviderModelRecord, enabled: boolean) => {
      if (!selected_record || pending_action || !selected_can_manage) {
        return;
      }
      if (model.is_default && !enabled) {
        set_feedback({
          tone: "error",
          title: t("settings.providers.default_model_disable_title"),
          message: t("settings.providers.default_model_disable_message", {
            model: model.display_name || model.model_id,
          }),
        });
        return;
      }
      try {
        set_pending_action(`model:${model.model_id}`);
        await update_provider_model_api(
          selected_record.provider,
          model.model_id,
          model_update_payload(model, { enabled }),
        );
        await refresh_all(selected_record.provider);
      } catch (error) {
        set_feedback({
          tone: "error",
          title: t("settings.providers.model_status_failed_title"),
          message:
            error instanceof Error
              ? error.message
              : t("settings.providers.retry_later"),
        });
      } finally {
        set_pending_action(null);
      }
    },
    [
      pending_action,
      refresh_all,
      selected_can_manage,
      selected_record,
      set_feedback,
      set_pending_action,
      t,
    ],
  );

  const handle_save_model_options = useCallback(async () => {
    if (!selected_record || !model_options || pending_action || !selected_can_manage) {
      return;
    }
    try {
      set_pending_action(`options:${model_options.model.model_id}`);
      const provider_options = parse_provider_options(
        model_options.provider_options_text,
        t("settings.providers.provider_options_json_object"),
      );
      await update_provider_model_api(
        selected_record.provider,
        model_options.model.model_id,
        model_update_payload(model_options.model, {
          capabilities_override: model_options.capabilities,
          context_window: model_options.context_window.trim()
            ? Number(model_options.context_window)
            : null,
          max_output_tokens: model_options.max_output_tokens.trim()
            ? Number(model_options.max_output_tokens)
            : null,
          provider_options,
        }),
      );
      set_model_options(null);
      await refresh_all(selected_record.provider);
    } catch (error) {
      set_feedback({
        tone: "error",
        title: t("settings.providers.model_options_save_failed_title"),
        message:
          error instanceof Error
            ? error.message
            : t("settings.providers.check_json_format"),
      });
    } finally {
      set_pending_action(null);
    }
  }, [
    model_options,
    pending_action,
    refresh_all,
    selected_can_manage,
    selected_record,
    set_feedback,
    set_pending_action,
    t,
  ]);

  return {
    add_model_open,
    displayed_models,
    handle_add_model,
    handle_fetch_models,
    handle_open_add_model,
    handle_save_model_options,
    handle_test_selection,
    handle_toggle_model,
    manual_model_enabled,
    manual_model_id,
    manual_model_placeholder,
    model_options,
    model_query,
    reset_model_controls,
    set_add_model_open,
    set_manual_model_enabled,
    set_manual_model_id,
    set_model_options,
    set_model_options_from_record: (model: ProviderModelRecord) =>
      set_model_options(model_options_from_record(model)),
    set_model_query,
    test_model_options,
  };
}
