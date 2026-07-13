import { useCallback, useMemo, useState, type FormEvent } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";

import {
  buildLoginPageState,
  getLoginSubmitError,
  resolveLoginRedirectPath,
} from "./login-page-model";

export function useLoginPageController() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const {
    error: authError,
    isBootstrapped,
    loading,
    login,
    refreshStatus,
    status,
  } = useAuth();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const redirectPath = useMemo(
    () => resolveLoginRedirectPath(searchParams.get("redirect")),
    [searchParams],
  );
  const pageState = useMemo(
    () => buildLoginPageState({
      isBootstrapped,
      loading,
      redirectPath,
      status,
    }),
    [isBootstrapped, loading, redirectPath, status],
  );

  const refresh = useCallback(() => {
    void refreshStatus().catch((error: unknown) => {
      console.warn("[LoginPage] Auth refresh failed:", error);
    });
  }, [refreshStatus]);

  const submit = useCallback(async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setIsSubmitting(true);
    setSubmitError(null);
    try {
      await login(username, password);
      navigate(redirectPath, { replace: true });
    } catch (error) {
      setSubmitError(getLoginSubmitError(error, t("login.unknown_error")));
    } finally {
      setIsSubmitting(false);
    }
  }, [login, navigate, password, redirectPath, t, username]);

  return {
    authError,
    isSubmitting,
    pageState,
    password,
    refresh,
    setPassword,
    setUsername,
    submit,
    submitError,
    username,
  };
}
