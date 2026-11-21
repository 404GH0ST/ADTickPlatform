"use client";

import {
  useCallback,
  useMemo,
  useRef,
  useState,
  type FormEvent,
  type KeyboardEvent,
  type RefCallback,
} from "react";

import { browserRequestErrorMessage, parseApiError } from "@/lib/api-utils";
import { keepEarlyInput } from "@/lib/early-input";

export type FlagVerdictOutcome = "accepted" | "duplicate" | "rejected";

export type FlagVerdict = {
  flag: string;
  status: string;
  outcome: FlagVerdictOutcome;
  detail?: string;
  message?: string;
};

export type FlagSubmitNote = {
  message: string;
  variant: "success" | "warning" | "error";
};

/**
 * Everything a flag form needs. The FlagSubmit theme slot receives exactly
 * this, so a themed form only decides how to draw it.
 */
export type FlagSubmitState = {
  rawFlags: string;
  setRawFlags: (value: string) => void;
  /** Number of flags parsed from rawFlags (newline or comma separated). */
  flagCount: number;
  pending: boolean;
  /** True when scoring is not accepting flags; disable the form. */
  closed: boolean;
  error: string | null;
  note: FlagSubmitNote | null;
  verdicts: FlagVerdict[];
  hasFeedback: boolean;
  /** Short helper line, e.g. "2 flags ready · ⌘/Ctrl+Enter". */
  hint: string;
  /** Use as a <form onSubmit>. */
  onSubmit: (event?: FormEvent<HTMLFormElement>) => void;
  /** Optional: submits on ⌘/Ctrl+Enter when attached to the textarea. */
  onKeyDown: (event: KeyboardEvent<HTMLTextAreaElement>) => void;
  /** Attach to the textarea: focuses it on errors and keeps text typed before the page loads. */
  inputRef: RefCallback<HTMLTextAreaElement>;
};

export function verdictOutcome(status: string): FlagVerdictOutcome {
  const normalized = status.trim().toLowerCase();
  if (normalized === "accepted") {
    return "accepted";
  }
  if (normalized === "duplicate") {
    return "duplicate";
  }
  return "rejected";
}

function summaryVariant(accepted: number, rejected: number) {
  if (accepted > 0 && rejected === 0) {
    return "success" as const;
  }
  if (accepted === 0) {
    return "error" as const;
  }
  return "warning" as const;
}

export function parseFlags(raw: string): string[] {
  return raw
    .split(/[\n,]+/)
    .map((value) => value.trim())
    .filter(Boolean);
}

export function useFlagSubmit({
  acceptingSubmissions,
}: {
  acceptingSubmissions?: boolean;
}): FlagSubmitState {
  const inputNode = useRef<HTMLTextAreaElement>(null);
  const [rawFlags, setRawFlagsState] = useState("");
  const inputRef = useCallback((node: HTMLTextAreaElement | null) => {
    inputNode.current = node;
    keepEarlyInput(setRawFlagsState)(node);
  }, []);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [note, setNote] = useState<FlagSubmitNote | null>(null);
  const [verdicts, setVerdicts] = useState<FlagVerdict[]>([]);

  const closed = acceptingSubmissions === false;
  const flagCount = useMemo(() => parseFlags(rawFlags).length, [rawFlags]);
  const hasFeedback = Boolean(error || note || verdicts.length > 0);

  function setRawFlags(value: string) {
    setRawFlagsState(value);
    if (error) {
      setError(null);
    }
  }

  async function submit() {
    const flags = parseFlags(rawFlags);
    if (flags.length === 0) {
      setError("Paste at least one flag.");
      setNote(null);
      setVerdicts([]);
      inputNode.current?.focus();
      return;
    }

    setPending(true);
    setError(null);
    setNote(null);
    setVerdicts([]);

    try {
      const response = await fetch("/api/platform/submit", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ flags }),
      });
      if (!response.ok) {
        throw new Error(await parseApiError(response, "/api/platform/submit"));
      }
      const payload = (await response.json()) as {
        results?: Omit<FlagVerdict, "outcome">[];
        accepted_count?: number;
        rejected_count?: number;
      };
      const results = (payload.results ?? []).map((verdict) => ({
        ...verdict,
        outcome: verdictOutcome(verdict.status),
      }));
      const acceptedCount = payload.accepted_count ?? 0;
      const rejectedCount = payload.rejected_count ?? 0;
      setVerdicts(results);
      setNote({
        message: `Accepted ${acceptedCount}, rejected ${rejectedCount}.`,
        variant: summaryVariant(acceptedCount, rejectedCount),
      });
      if (acceptedCount > 0) {
        setRawFlagsState("");
        window.dispatchEvent(new Event("platform:flag-accepted"));
      }
    } catch (submitError) {
      setError(browserRequestErrorMessage(submitError, "Flag submit failed."));
    } finally {
      setPending(false);
    }
  }

  function onSubmit(event?: FormEvent<HTMLFormElement>) {
    event?.preventDefault();
    if (pending || closed) {
      return;
    }
    void submit();
  }

  function onKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key !== "Enter" || (!event.metaKey && !event.ctrlKey)) {
      return;
    }
    event.preventDefault();
    if (pending || closed) {
      return;
    }
    event.currentTarget.form?.requestSubmit();
  }

  const hint = closed
    ? "Scoring is not accepting flags right now."
    : flagCount > 0
      ? `${flagCount} flag${flagCount === 1 ? "" : "s"} ready · ⌘/Ctrl+Enter`
      : "One per line or comma-separated · ⌘/Ctrl+Enter";

  return {
    rawFlags,
    setRawFlags,
    flagCount,
    pending,
    closed,
    error,
    note,
    verdicts,
    hasFeedback,
    hint,
    onSubmit,
    onKeyDown,
    inputRef,
  };
}
