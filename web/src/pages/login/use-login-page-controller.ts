// INPUT: Auth status, credential drafts and validated local redirect.
// OUTPUT: Login page state and synchronously exclusive authentication commands.
// POS: Login controller; never replays a pending sign-in automatically.

import { useCallback, useMemo, useRef, useState, type FormEvent } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

import { useAuth } from "@/shared/auth/auth-context";
import { registerApi } from "@/lib/api/account/auth-api";
import { useI18n } from "@/shared/i18n/i18n-context";

import {
  buildLoginStatusFailure,
  buildLoginSubmitFailure,
  buildLoginPageState,
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
  const [registering, setRegistering] = useState(false);
  const [submitFailure, setSubmitFailure] = useState<ReturnType<
    typeof buildLoginSubmitFailure
  > | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const submittingRef = useRef(false);
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
    void refreshStatus()
      .catch((error: unknown) => {
        console.warn("[LoginPage] Auth refresh failed:", error);
      });
  }, [refreshStatus]);

  const submit = useCallback(async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (submittingRef.current) return;
    submittingRef.current = true;
    setIsSubmitting(true);
    setSubmitFailure(null);
    try {
      if (registering && status?.registration_enabled) {
        await registerApi({ username, password });
        await refreshStatus();
      } else {
        await login(username, password);
      }
      navigate(redirectPath, { replace: true });
    } catch (error) {
      setSubmitFailure(buildLoginSubmitFailure(error, t));
    } finally {
      submittingRef.current = false;
      setIsSubmitting(false);
    }
  }, [login, navigate, password, redirectPath, t, username, registering, status?.registration_enabled, refreshStatus]);

  return {
    registering,
    setRegistering,
    registrationEnabled: status?.registration_enabled === true,
    authFailure: authError
      ? buildLoginStatusFailure(authError, status !== null, t)
      : null,
    isSubmitting,
    pageState,
    password,
    refresh,
    setPassword: (value: string) => {
      setPassword(value);
      setSubmitFailure(null);
    },
    setUsername: (value: string) => {
      setUsername(value);
      setSubmitFailure(null);
    },
    submit,
    submitFailure,
    username,
  };
}
