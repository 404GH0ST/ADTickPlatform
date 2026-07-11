"use client";

import { useState, type FormEvent, type ReactElement } from "react";
import { Flag, LoaderCircle } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { StatusBanner } from "@/components/ui/status-banner";
import { parseApiError } from "@/lib/api-utils";

type Verdict = {
  flag: string;
  status: string;
  detail?: string;
  message?: string;
};

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

  return (
    <Card data-testid="flag-submit-panel">
      <CardHeader>
        <div className="flex items-center gap-2">
          <Flag className="h-4 w-4 text-primary" />
          <CardTitle className="text-base">Submit flags</CardTitle>
        </div>
        <CardDescription>
          Paste one flag per line (or comma-separated). Uses the same
          bulk-submit contract as the participant API.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form className="grid gap-3" onSubmit={onSubmit}>
          {acceptingSubmissions === false ? (
            <StatusBanner
              message="Submissions are currently closed (match not accepting flags)."
              variant="warning"
            />
          ) : null}
          <label className="grid gap-1.5 text-sm font-medium" htmlFor="flag-batch">
            Flags
            <textarea
              id="flag-batch"
              className="min-h-28 w-full rounded-sm border border-input bg-background px-3 py-2 font-mono text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring"
              placeholder={"PLAYIT{...}\nPLAYIT{...}"}
              value={rawFlags}
              onChange={(event) => setRawFlags(event.target.value)}
              disabled={pending}
            />
          </label>
          {error ? <StatusBanner message={error} variant="error" /> : null}
          {note ? <StatusBanner message={note} variant="success" /> : null}
          {verdicts.length > 0 ? (
            <ul className="grid gap-1 rounded-sm border bg-muted/20 p-3 text-xs">
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
          <div>
            <Button type="submit" disabled={pending || acceptingSubmissions === false}>
              {pending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
              {pending ? "Submitting..." : "Submit flags"}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
