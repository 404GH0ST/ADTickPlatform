"use client";

import { useState, type FormEvent, type ReactElement } from "react";
import { Flag, LoaderCircle } from "lucide-react";

import { Button } from "@/components/ui/button";
import { StatusBanner } from "@/components/ui/status-banner";
import { parseApiError } from "@/lib/api-utils";

type Verdict = {
  flag: string;
  status: string;
  detail?: string;
  message?: string;
};

/**
 * Global attack-flag entry: sits under the participant nav so submit is always
 * reachable (not buried under service controls).
 */
export function FlagSubmitPanel({
  acceptingSubmissions,
}: {
  acceptingSubmissions?: boolean;
}): ReactElement {
  const [rawFlags, setRawFlags] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [note, setNote] = useState<string | null>(null);
  const [verdicts, setVerdicts] = useState<Verdict[]>([]);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const flags = rawFlags
      .split(/[\n,]+/)
      .map((value) => value.trim())
      .filter(Boolean);
    if (flags.length === 0) {
      setError("Paste at least one flag.");
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
          : "flag submit failed",
      );
    } finally {
      setPending(false);
    }
  }

  const closed = acceptingSubmissions === false;

  return (
    <section
      data-testid="flag-submit-panel"
      className="surface-workroom rounded-sm border border-border bg-card p-3"
      aria-label="Submit flags"
    >
      <form className="grid gap-2" onSubmit={onSubmit}>
        <div className="flex flex-wrap items-center gap-2">
          <Flag className="h-4 w-4 shrink-0 text-primary" aria-hidden />
          <p className="text-sm font-semibold">Submit flags</p>
          {closed ? (
            <span className="text-xs text-highlight">
              Submissions closed
            </span>
          ) : (
            <span className="text-xs text-muted-foreground">
              One per line or comma-separated
            </span>
          )}
        </div>
        <div className="flex flex-col gap-2 sm:flex-row sm:items-stretch">
          <label className="sr-only" htmlFor="flag-batch">
            Flags
          </label>
          <textarea
            id="flag-batch"
            className="min-h-11 w-full flex-1 resize-y rounded-sm border border-input bg-background px-3 py-2 font-mono text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background sm:min-h-[2.75rem]"
            rows={2}
            placeholder="PLAYIT{...}"
            value={rawFlags}
            onChange={(event) => setRawFlags(event.target.value)}
            disabled={pending || closed}
          />
          <Button
            type="submit"
            className="shrink-0 sm:self-start"
            disabled={pending || closed}
          >
            {pending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
            {pending ? "Submitting..." : "Submit"}
          </Button>
        </div>
        {error ? <StatusBanner message={error} variant="error" /> : null}
        {note ? <StatusBanner message={note} variant="success" /> : null}
        {verdicts.length > 0 ? (
          <ul className="grid max-h-40 gap-1 overflow-y-auto rounded-sm border bg-muted/20 p-2 text-xs">
            {verdicts.map((verdict, index) => (
              <li key={`${verdict.flag}-${index}`} className="font-mono">
                <span
                  className={
                    verdict.status.toLowerCase().includes("accept")
                      ? "text-positive"
                      : "text-negative"
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
            ))}
          </ul>
        ) : null}
      </form>
    </section>
  );
}
