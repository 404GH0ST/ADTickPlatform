"use client";

import {
  useMemo,
  useRef,
  useState,
  type FormEvent,
  type KeyboardEvent,
  type ReactElement,
} from "react";
import { Flag, LoaderCircle } from "lucide-react";

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
  const [note, setNote] = useState<string | null>(null);
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
      setVerdicts(results);
      setNote(
        `Accepted ${payload.accepted_count ?? 0}, rejected ${payload.rejected_count ?? 0}.`,
      );
      if ((payload.accepted_count ?? 0) > 0) {
        setRawFlags("");
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
            {note ? <StatusBanner message={note} variant="success" /> : null}
            {verdicts.length > 0 ? (
              <ul
                className="grid max-h-40 gap-1 overflow-y-auto rounded-sm border border-border bg-muted/20 p-2 text-xs"
                aria-label="Flag verdicts"
              >
                {verdicts.map((verdict, index) => {
                  const accepted = verdict.status
                    .toLowerCase()
                    .includes("accept");
                  return (
                    <li
                      key={`${verdict.flag}-${index}`}
                      className="break-all font-mono leading-5"
                    >
                      <span
                        className={
                          accepted ? "text-positive" : "text-negative"
                        }
                      >
                        {verdict.status}
                      </span>
                      {": "}
                      {verdict.flag}
                      {verdict.detail || verdict.message
                        ? ` — ${verdict.detail || verdict.message}`
                        : ""}
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
