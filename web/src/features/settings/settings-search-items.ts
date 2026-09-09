// INPUT: 设置模块及现有本地化设置项名称、说明。
// OUTPUT: 搜索可见设置内容的静态目录，不包含用户输入或凭据。
// POS: 设置内容搜索索引；新增设置项在此登记，复用原始文案键。
import type { TranslationKey } from "@/shared/i18n/messages";
import type { SettingsSectionKey } from "./settings-navigation-model";

export const SETTINGS_SEARCH_ITEMS: Record<SettingsSectionKey, readonly (readonly TranslationKey[])[]> = {
  general: [
    ["settings.desktop.version_title"],
    ["settings.desktop.export_logs"],
    ["settings.general.default_model_title", "settings.general.default_model_description"],
    ["settings.general.default_background_model_title", "settings.general.default_background_model_description"],
    ["settings.general.default_image_model_title", "settings.general.default_image_model_description"],
    ["settings.general.default_vision_model_title", "settings.general.default_vision_model_description"],
    ["settings.general.runtime_title", "settings.general.runtime_description"],
    ["settings.general.auto_memory_title", "settings.general.auto_memory_description"],
    ["settings.general.auto_dream_title", "settings.general.auto_dream_description"],
    ["settings.general.emotion_title", "settings.general.emotion_description"],
    ["settings.general.echo_title", "settings.general.echo_description"],
    ["settings.general.agent_sdk_diagnostics_title", "settings.general.agent_sdk_diagnostics_description"],
    ["settings.onboarding_title", "settings.onboarding_description"],
  ],
  appearance: [
    ["theme.switch_title", "settings.general.theme_description"],
    ["language.switch_title", "settings.general.language_description"],
    ["settings.reading.font"],
    ["settings.reading.size"],
    ["settings.reading.spacing"],
    ["settings.appearance.reset", "settings.appearance.reset_description"],
  ],
  permissions: [
    ["settings.general.agent_defaults_title", "settings.general.agent_defaults_description", "agent_options.advanced.permission.default.description", "agent_options.advanced.permission.plan.description", "agent_options.advanced.permission.auto.description", "agent_options.advanced.permission.accept_edits.description", "agent_options.advanced.permission.bypass.description", "agent_options.advanced.permission.dont_ask.description"],
  ],
  workspace: [
    ["settings.general.state_root_title", "settings.general.state_root_description"],
  ],
  runtime: [
    ["settings.runtime.kernel_title", "settings.runtime.kernel_description"],
    ["settings.runtime.tool_search_title", "settings.runtime.tool_search_description"],
    ["settings.runtime.web_search_title", "settings.runtime.web_search_description"],
    ["settings.runtime.web_search_provider"],
    ["settings.runtime.web_search_api_key"],
    ["settings.runtime.web_search_base_url"],
    ["settings.runtime.web_search_custom_base_url"],
    ["settings.runtime.web_search_country"],
    ["settings.runtime.web_search_language"],
    ["settings.runtime.web_search_search_language"],
    ["settings.runtime.web_search_freshness"],
    ["settings.runtime.web_search_result_count"],
    ["settings.runtime.web_search_timeout"],
    ["settings.runtime.web_search_cache"],
    ["settings.runtime.web_search_depth"],
    ["settings.runtime.web_search_extract_depth"],
    ["settings.runtime.web_search_private_network"],
    ["settings.runtime.web_search_provider_extract"],
    ["settings.runtime.web_search_anysearch_domain"],
    ["settings.runtime.web_search_anysearch_tag"],
    ["settings.runtime.web_search_anysearch_content_types"],
    ["settings.runtime.web_search_anysearch_params"],
  ],
  browser: [
    ["settings.browser.install_action"],
    ["settings.browser.cdp_title", "settings.browser.cdp_description"],
  ],
  personal: [
    ["settings.personal.role"],
    ["settings.personal.auth_method"],
    ["settings.personal.quota_limit"],
    ["settings.personal.avatar_alt"],
    ["settings.personal.password_title"],
    ["settings.personal.token_usage_title"],
  ],
  providers: [
    ["settings.providers.provider_name"],
    ["settings.providers.api_format", "settings.providers.api_format_runtime_hint"],
    ["settings.providers.api_key"],
    ["settings.providers.models"],
    ["settings.providers.ccswitch_title"],
    ["settings.providers.section_title", "settings.providers.section_description"],
  ],
  operations: [
    ["operations.tabs.members"],
    ["operations.tabs.user_subscriptions"],
    ["operations.tabs.subscription_plans"],
    ["operations.tabs.subscription_providers"],
    ["operations.tabs.projects"],
  ],
};
