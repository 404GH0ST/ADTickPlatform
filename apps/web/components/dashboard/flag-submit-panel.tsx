"use client";

import {
  useMemo,
  useRef,
  useState,
  type FormEvent,
  type KeyboardEvent,
  type ReactElement,
} from "react";
import { CircleCheck, CircleMinus, CircleX, Flag, LoaderCircle } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { StatusBanner } from "@/components/ui/status-banner";
import { parseApiError } from "@/lib/api-utils";
import { cn } from "@/lib/utils";

type Verdict = {
  flag: string;
  status: string;
  detail?: string;
  message?: string;
};

type VerdictOutcome = "accepted" | "duplicate" | "rejected";

const verdictPresentation: Record<
  VerdictOutcome,
  { label: string; tone: string; Icon: typeof CircleCheck }
> = {
  accepted: { label: "Accepted", tone: "tone-success", Icon: CircleCheck },
  duplicate: { label: "Duplicate", tone: "tone-warning", Icon: CircleMinus },
  rejected: { label: "Rejected", tone: "tone-danger", Icon: CircleX },
};

function verdictOutcome(status: string): VerdictOutcome {
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

function parseFlags(raw: string): string[] {
  return raw
    .split(/[\n,]+/)
    .map((value) => value.trim())
    .filter(Boolean);
}

/**
 * Global attack-flag entry: sits under the participant nav so submit is always
 * reachable (not buried under service controls).
 */
export function FlagSubmitPanel({
  acceptingSubmissions,
}: {
  acceptingSubmissions?: boolean;
}): ReactElement {
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const [rawFlags, setRawFlags] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [note, setNote] = useState<{
    message: string;
    variant: "success" | "warning" | "error";
  } | null>(null);
  const [verdicts, setVerdicts] = useState<Verdict[]>([]);

  const closed = acceptingSubmissions === false;
  const flagCount = useMemo(() => parseFlags(rawFlags).length, [rawFlags]);
  const hasFeedback = Boolean(error || note || verdicts.length > 0);
  const feedbackId = "flag-submit-feedback";
  const hintId = "flag-submit-hint";

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const flags = parseFlags(rawFlags);
    if (flags.length === 0) {
      setError("Paste at least one flag.");
      setNote(null);
      setVerdicts([]);
      textareaRef.current?.focus();
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
        results?: Verdict[];
        accepted_count?: number;
        rejected_count?: number;
      };
      const results = payload.results ?? [];
      const acceptedCount = payload.accepted_count ?? 0;
      const rejectedCount = payload.rejected_count ?? 0;
      setVerdicts(results);
      setNote({
        message: `Accepted ${acceptedCount}, rejected ${rejectedCount}.`,
        variant: summaryVariant(acceptedCount, rejectedCount),
      });
      if (acceptedCount > 0) {
        setRawFlags("");
        window.dispatchEvent(new Event("platform:flag-accepted"));
      }
    } catch (submitError) {
      setError(
        submitError instanceof Error
          ? submitError.message
          : "Flag submit failed.",
      );
    } finally {
      setPending(false);
    }
  }

  function onTextareaKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key !== "Enter" || (!event.metaKey && !event.ctrlKey)) {
      return;
    }
    event.preventDefault();
    if (pending || closed) {
      return;
    }
    event.currentTarget.form?.requestSubmit();
  }

  const railHint = closed
    ? "Scoring is not accepting flags right now."
    : flagCount > 0
      ? `${flagCount} flag${flagCount === 1 ? "" : "s"} ready · ⌘/Ctrl+Enter`
      : "One per line or comma-separated · ⌘/Ctrl+Enter";

  return (
    <section
      data-testid="flag-submit-panel"
      className="surface-workroom rounded-sm border border-border bg-card p-3"
      aria-label="Submit flags"
    >
      <form
        className="grid gap-3"
        onSubmit={onSubmit}
        aria-busy={pending || undefined}
      >
        <div className="grid gap-2">
          <div className="flex flex-wrap items-center gap-2">
            <Flag className="h-4 w-4 shrink-0 text-primary" aria-hidden />
            <h2 className="text-sm font-semibold">Submit flags</h2>
            {closed ? (
              <Badge className="tone-warning" variant="outline">
                Submissions closed
              </Badge>
            ) : null}
          </div>

          <div
            className={cn(
              "overflow-hidden rounded-sm border border-input bg-background transition-[border-color,box-shadow]",
              "focus-within:border-ring focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2 focus-within:ring-offset-background",
              error &&
                !pending &&
                "border-destructive focus-within:border-destructive focus-within:ring-destructive/40",
            )}
          >
            <label className="sr-only" htmlFor="flag-batch">
              Flags
            </label>
            <textarea
              ref={textareaRef}
              id="flag-batch"
              className="min-h-11 w-full resize-y border-0 bg-transparent px-3 py-2 font-mono text-sm text-foreground outline-none placeholder:text-muted-foreground disabled:cursor-not-allowed disabled:opacity-60"
              rows={2}
              placeholder="PLAYIT{...}"
              value={rawFlags}
              onChange={(event) => {
                setRawFlags(event.target.value);
                if (error) {
                  setError(null);
                }
              }}
              onKeyDown={onTextareaKeyDown}
              disabled={pending || closed}
              spellCheck={false}
              autoComplete="off"
              aria-invalid={error ? true : undefined}
              aria-describedby={
                hasFeedback ? `${hintId} ${feedbackId}` : hintId
              }
            />
            <div className="flex flex-wrap items-center justify-between gap-2 border-t border-border bg-muted/20 px-2 py-1.5">
              <p
                id={hintId}
                className="min-w-0 text-xs leading-5 text-muted-foreground"
              >
                {railHint}
              </p>
              <Button
                type="submit"
                size="sm"
                className="min-h-8 min-w-[5.5rem] shrink-0"
                disabled={pending || closed}
              >
                {pending ? (
                  <LoaderCircle className="h-4 w-4 animate-spin" aria-hidden />
                ) : null}
                <span>{pending ? "Submitting…" : "Submit"}</span>
              </Button>
            </div>
          </div>
        </div>

        {hasFeedback ? (
          <div id={feedbackId} className="grid gap-2" aria-live="polite">
            {error ? <StatusBanner message={error} variant="error" /> : null}
            {note ? (
              <div data-testid="flag-submit-summary" data-variant={note.variant}>
                <StatusBanner message={note.message} variant={note.variant} />
              </div>
            ) : null}
            {verdicts.length > 0 ? (
              <ul
                className="grid max-h-40 gap-1 overflow-y-auto text-xs"
                aria-label="Flag verdicts"
              >
                {verdicts.map((verdict, index) => {
                  const outcome = verdictOutcome(verdict.status);
                  const { label, tone, Icon } = verdictPresentation[outcome];
                  const detail = verdict.detail || verdict.message;
                  return (
                    <li
                      key={`${verdict.flag}-${index}`}
                      data-testid="flag-verdict"
                      data-verdict={outcome}
                      className={cn(
                        "flex items-start gap-2 rounded-sm border px-2 py-1 leading-5",
                        tone,
                      )}
                    >
                      <Icon className="mt-0.5 h-3.5 w-3.5 shrink-0" aria-hidden />
                      <span className="min-w-0 break-all">
                        <span className="font-semibold">{label}</span>
                        {": "}
                        <span className="font-mono">{verdict.flag}</span>
                        {detail ? ` — ${detail}` : ""}
                      </span>
                    </li>
                  );
                })}
              </ul>
            ) : null}
          </div>
        ) : null}
      </form>
    </section>
  );
}
