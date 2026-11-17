"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";

import { parseApiError } from "@/lib/api-utils";

export type ParticipantLoginMode = "login" | "register";

/**
 * Login / registration state and actions. The Login theme slot receives
 * exactly this, so a themed form only decides how to draw it.
 */
export type ParticipantLoginState = {
  mode: ParticipantLoginMode;
  /** Switches mode and clears any error. */
  setMode: (mode: ParticipantLoginMode) => void;
  displayName: string;
  setDisplayName: (value: string) => void;
  email: string;
  setEmail: (value: string) => void;
  password: string;
  setPassword: (value: string) => void;
  pending: boolean;
  error: string | null;
  /** Minimum password length enforced on registration. */
  minPasswordLength: number;
  /** Use as a <form onSubmit>. Redirects to /services on success. */
  onSubmit: (event?: FormEvent<HTMLFormElement>) => void;
};

export function useParticipantLogin(): ParticipantLoginState {
  const router = useRouter();
  const [mode, setModeState] = useState<ParticipantLoginMode>("login");
  const [displayName, setDisplayName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function setMode(next: ParticipantLoginMode) {
    setModeState(next);
    setError(null);
  }

  async function submit() {
    setPending(true);
    setError(null);

    try {
      const endpoint =
        mode === "register" ? "/api/platform/session/register" : "/api/platform/session/login";
      const payload =
        mode === "register" ? { display_name: displayName, email, password } : { email, password };
      const response = await fetch(endpoint, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });

      if (!response.ok) {
        throw new Error(await parseApiError(response, endpoint));
      }

      router.push("/services");
      router.refresh();
    } catch (submitError) {
      setError(
        submitError instanceof Error
          ? submitError.message
          : mode === "register"
            ? "player registration failed"
            : "participant login failed",
      );
    } finally {
      setPending(false);
    }
  }

  function onSubmit(event?: FormEvent<HTMLFormElement>) {
    event?.preventDefault();
    if (pending) {
      return;
    }
    void submit();
  }

  return {
    mode,
    setMode,
    displayName,
    setDisplayName,
    email,
    setEmail,
    password,
    setPassword,
    pending,
    error,
    minPasswordLength: 8,
    onSubmit,
  };
}
