"use client";

import { useState, type FormEvent, type ReactElement } from "react";
import { LoaderCircle, Upload } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { StatusBanner } from "@/components/ui/status-banner";
import { processApiResponse, parseApiError } from "@/lib/api-utils";
import type { AdminBulkImportResult } from "@/lib/admin-dashboard-types";
import { parseTeamCsv } from "@/lib/team-csv";

export function BulkImportPanel(): ReactElement {
  const [csvText, setCsvText] = useState(
    "team_name,contact_email,player_display_name,player_email,player_password,player_role\n",
  );
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [note, setNote] = useState<string | null>(null);
  const [details, setDetails] = useState<string[]>([]);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setError(null);
    setNote(null);
    setDetails([]);

    try {
      const teams = parseTeamCsv(csvText);
      if (teams.length === 0) {
        throw new Error("No valid team rows found in CSV.");
      }
      const response = await fetch("/api/admin/import/teams", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ teams }),
      });
      // 200 = at least one team created; 422 = structured all-failed result.
      if (response.status === 422) {
        const result = (await response.json()) as AdminBulkImportResult;
        setError(
          `Import created no teams. ${(result.errors ?? []).join(" ")}`.trim(),
        );
        setDetails(result.errors ?? []);
        return;
      }
      if (!response.ok) {
        throw new Error(await parseApiError(response, "/api/admin/import/teams"));
      }
      const result = await processApiResponse<AdminBulkImportResult>(
        response,
        "/api/admin/import/teams",
      );
      setNote(
        `Created ${result.teams_created} teams and ${result.players_created} players.`,
      );
      setDetails(result.errors ?? []);
    } catch (importError) {
      setError(
        importError instanceof Error ? importError.message : "import failed",
      );
    } finally {
      setPending(false);
    }
  }

  return (
    <Card data-testid="bulk-import-panel">
      <CardHeader>
        <div className="flex items-center gap-2">
          <Upload className="h-4 w-4 text-primary" />
          <CardTitle className="text-base">Bulk team import</CardTitle>
        </div>
        <CardDescription>
          CSV columns: team_name, contact_email, optional player_display_name,
          player_email, player_password, player_role. Each team is created
          atomically with its players; a bad player row rolls that team back.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form className="grid gap-3" onSubmit={onSubmit}>
          <textarea
            className="min-h-36 w-full rounded-sm border border-input bg-background px-3 py-2 font-mono text-xs outline-none focus-visible:ring-2 focus-visible:ring-ring"
            value={csvText}
            onChange={(event) => setCsvText(event.target.value)}
          />
          {error ? <StatusBanner message={error} variant="error" /> : null}
          {note ? <StatusBanner message={note} variant="success" /> : null}
          {details.length > 0 ? (
            <ul className="rounded-sm border bg-muted/20 p-3 text-xs text-muted-foreground">
              {details.map((line) => (
                <li key={line}>{line}</li>
              ))}
            </ul>
          ) : null}
          <div>
            <Button type="submit" disabled={pending}>
              {pending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
              Import teams
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
