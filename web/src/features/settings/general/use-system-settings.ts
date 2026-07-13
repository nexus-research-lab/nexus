import { useEffect, useState } from "react";

import {
  getSystemVersionApi,
  type SystemVersionInfo,
} from "@/lib/api/settings/system-api";
import { getErrorMessage } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";

const DEFAULT_RELEASE_PAGE_URL =
  "https://github.com/nexus-research-lab/nexus/releases/latest";

function describeSystemVersion(
  version: SystemVersionInfo | null,
  loading: boolean,
  messages: {
    loading: string;
    unavailable: string;
    value: string;
  },
): string {
  if (version) {
    return messages.value
      .replace("{version}", version.version)
      .replace("{target}", version.target);
  }
  return loading ? messages.loading : messages.unavailable;
}

export function useSystemSettings() {
  const { t } = useI18n();
  const [version, setVersion] = useState<SystemVersionInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [feedbackMessage, setFeedbackMessage] = useState("");

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    void getSystemVersionApi()
      .then((result) => {
        if (!cancelled) {
          setVersion(result);
          setFeedbackMessage("");
        }
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setFeedbackMessage(getErrorMessage(
            error,
            t("settings.system.version_failed"),
          ));
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [t]);

  return {
    feedbackMessage,
    releasePageUrl: version?.release_url || DEFAULT_RELEASE_PAGE_URL,
    versionDescription: describeSystemVersion(version, loading, {
      loading: t("settings.system.version_loading"),
      unavailable: t("settings.system.version_unavailable"),
      value: t("settings.system.version_value"),
    }),
  };
}
