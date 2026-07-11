"use client";

import type { ReactElement } from "react";
import { Download } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { downloadText, toCsv } from "@/lib/team-csv";

export function ExportDataPanel({
  scoreboard,
  attacks,
}: {
  scoreboard: Array<{
    rank: number;
    team: string;
    attack: number;
    defense: number;
    sla: number;
    total: number;
    delta: string;
  }>;
  attacks: Array<{
    id: string;
    attacker: string;
    victim: string;
    service: string;
    tick: number;
    verdict: string;
  }>;
}): ReactElement {
  return (
    <Card data-testid="export-data-panel">
      <CardHeader>
        <div className="flex items-center gap-2">
          <Download className="h-4 w-4 text-primary" />
          <CardTitle className="text-base">Export snapshots</CardTitle>
        </div>
        <CardDescription>
          Download the scoreboard and attack rows currently loaded on this page
          (page slice, not necessarily the full match history).
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-wrap gap-2">
        <Button
          type="button"
          variant="outline"
          onClick={() => {
            const csv = toCsv([
              ["rank", "team", "attack", "defense", "sla", "total", "delta"],
              ...scoreboard.map((row) => [
                String(row.rank),
                row.team,
                String(row.attack),
                String(row.defense),
                String(row.sla),
                String(row.total),
                row.delta,
              ]),
            ]);
            downloadText(
              `scoreboard-${new Date().toISOString().slice(0, 10)}.csv`,
              csv,
            );
          }}
        >
          Export scoreboard CSV
        </Button>
        <Button
          type="button"
          variant="outline"
          onClick={() => {
            const csv = toCsv([
              ["id", "attacker", "victim", "service", "tick", "verdict"],
              ...attacks.map((row) => [
                row.id,
                row.attacker,
                row.victim,
                row.service,
                String(row.tick),
                row.verdict,
              ]),
            ]);
            downloadText(
              `attacks-${new Date().toISOString().slice(0, 10)}.csv`,
              csv,
            );
          }}
        >
          Export attacks CSV
        </Button>
      </CardContent>
    </Card>
  );
}
